// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";
import "./LendingPool.sol";

/// @title GlobalLedger (Centralized Liquidity and Logic Hub)
/// @notice This contract manages all collateral, loans, and liquidity for the lending protocol.
/// It also handles the deployment and registration of LendingPool contracts across different shards.
/// @dev Acts as the central state and execution core. LendingPool contracts interact with this contract
/// via asynchronous calls to handle user deposits, borrows, and repayments.
contract GlobalLedger is NilTokenBase {
    /// @notice The address of the deployer, allowed to call administrative functions.
    address public deployer;
    /// @notice The address of the InterestManager contract used to fetch interest rates.
    address public interestManager;
    /// @notice The address of the Oracle contract used to fetch token prices (indirectly via LendingPool).
    address public oracle;
    /// @notice The TokenId representing the USDT token.
    TokenId public usdt;
    /// @notice The TokenId representing the ETH token.
    TokenId public eth;

    // --- Errors --- //
    /// @dev Reverts if a function requiring deployer privileges is called by another address.
    error OnlyDeployer();
    /// @dev Reverts if trying to register or use an invalid (zero) pool address.
    error InvalidPoolAddress();
    /// @dev Reverts if trying to register a pool on a shard that already has one, or if the pool address is already registered.
    error PoolAlreadyRegistered();
    /// @dev Reverts if a function requiring a registered LendingPool caller is called by another address.
    error UnauthorizedCaller();
    /// @dev Reverts during borrow if the user's collateral balance is less than required.
    error InsufficientCollateral();
    /// @dev Reverts during borrow if the GlobalLedger lacks sufficient liquidity of the requested token.
    error InsufficientLiquidity();
    /// @dev Reverts during repayment if the amount sent is less than the required repayment (principal + interest).
    error RepaymentInsufficient();
    /// @dev Reverts during repayment if the user has no active loan or is repaying the wrong token.
    error NoActiveLoan();
    /// @dev Reverts if an expected cross-shard call (e.g., to Oracle, InterestManager) fails during processing.
    error CrossShardCallFailed(string message);
    /// @dev Reverts during deployment if an invalid shard ID (0 or >= 0xFFFF) is provided.
    error ShardIdInvalid();

    // --- Events --- //
    /// @notice Emitted when a LendingPool address is successfully registered for a specific shard.
    /// @param poolAddress The address of the registered LendingPool contract.
    /// @param shardId The shard ID the pool is registered for.
    event PoolRegistered(address indexed poolAddress, uint256 shardId);
    /// @notice Emitted when a new LendingPool contract is successfully deployed via `deployLendingPools`.
    /// @param poolAddress The address of the newly deployed LendingPool contract.
    /// @param shardId The shard ID the pool was deployed onto.
    event LendingPoolDeployed(address indexed poolAddress, uint256 shardId);
    /// @notice Emitted when a deposit forwarded from a LendingPool is successfully processed.
    /// @param user The original depositor's address.
    /// @param token The TokenId of the deposited asset.
    /// @param amount The amount deposited.
    event DepositHandled(address indexed user, TokenId token, uint256 amount);
    /// @notice Emitted when a borrow request is successfully processed and funds are sent.
    /// @param borrower The borrower's address.
    /// @param token The TokenId of the borrowed asset.
    /// @param amount The amount borrowed.
    event BorrowProcessed(
        address indexed borrower,
        TokenId token,
        uint256 amount
    );
    /// @notice Emitted when a loan repayment is successfully processed (loan record cleared).
    /// @param borrower The borrower's address.
    /// @param token The TokenId of the repaid asset (loan token).
    /// @param amount The principal amount of the loan that was cleared.
    event RepaymentProcessed(
        address indexed borrower,
        TokenId token,
        uint256 amount
    );
    /// @notice Emitted when collateral is successfully returned to the borrower after repayment.
    /// @param borrower The borrower's address.
    /// @param token The TokenId of the collateral asset returned.
    /// @param amount The amount of collateral returned.
    event CollateralReturned(
        address indexed borrower,
        TokenId token,
        uint256 amount
    );

    // --- State Variables --- //
    /// @notice Mapping from shard ID to the registered LendingPool address for that shard.
    mapping(uint256 => address) public lendingPoolsByShard;
    /// @notice Mapping from an address to a boolean indicating if it is a registered LendingPool.
    /// @dev Provides a quick lookup for the `onlyRegisteredLendingPool` modifier.
    mapping(address => bool) public isLendingPool;

    /// @notice Mapping storing user collateral balances.
    /// @dev `collateralBalances[userAddress][tokenId] = amount`
    mapping(address => mapping(TokenId => uint256)) public collateralBalances;

    /// @notice Mapping storing active loan details for each user.
    /// @dev `loans[userAddress] = Loan({amount, token})`
    mapping(address => Loan) public loans;

    /// @dev Struct to store loan details.
    struct Loan {
        uint256 amount; // Principal amount borrowed
        TokenId token; // Token borrowed
    }

    // --- Modifiers --- //
    /// @dev Ensures that the caller is the deployer address.
    modifier onlyDeployer() {
        if (msg.sender != deployer) revert OnlyDeployer();
        _;
    }

    /// @dev Ensures that the caller is a registered LendingPool contract.
    modifier onlyRegisteredLendingPool() {
        if (!isLendingPool[msg.sender]) revert UnauthorizedCaller();
        _;
    }

    /// @notice Initializes the GlobalLedger contract.
    /// @param _interestManager Address of the InterestManager contract.
    /// @param _oracle Address of the Oracle contract.
    /// @param _usdt TokenId for USDT.
    /// @param _eth TokenId for ETH.
    constructor(
        address _interestManager,
        address _oracle,
        TokenId _usdt,
        TokenId _eth
    ) {
        deployer = msg.sender;
        interestManager = _interestManager;
        oracle = _oracle;
        usdt = _usdt;
        eth = _eth;
    }

    /// @notice Manually registers a LendingPool contract for a specific shard.
    /// @dev Can only be called by the deployer. Primarily used if pools are deployed separately.
    /// @param poolAddress The address of the LendingPool to register.
    function registerLendingPool(address poolAddress) public onlyDeployer {
        if (poolAddress == address(0)) revert InvalidPoolAddress();

        uint256 shardId = Nil.getShardId(poolAddress);
        // Check if shard already has a pool or if address is already registered
        if (
            lendingPoolsByShard[shardId] != address(0) ||
            isLendingPool[poolAddress]
        ) {
            revert PoolAlreadyRegistered();
        }

        lendingPoolsByShard[shardId] = poolAddress;
        isLendingPool[poolAddress] = true;
        emit PoolRegistered(poolAddress, shardId);
    }

    /// @notice Deploys LendingPool contracts to the specified shards using `Nil.asyncDeploy`.
    /// @dev Automatically registers the deployed pools. Skips shards where a pool is already registered.
    /// Can only be called by the deployer.
    /// @param shardIds An array of shard IDs to deploy LendingPool contracts onto.
    function deployLendingPools(uint[] calldata shardIds) public onlyDeployer {
        for (uint i = 0; i < shardIds.length; i++) {
            uint shardId = shardIds[i];

            // Check if a pool already exists for this shard
            if (lendingPoolsByShard[shardId] != address(0)) {
                continue; // Skip deployment for this shard
            }

            if (shardId == 0 || shardId >= 0xFFFF) revert ShardIdInvalid();

            // Generate a deterministic salt for the deployment
            bytes32 salt = keccak256(
                abi.encodePacked(deployer, shardId, address(this))
            );

            // ABI-encode constructor arguments for LendingPool
            bytes memory constructorArgs = abi.encode(
                address(this), // _centralLedger
                interestManager,
                oracle,
                usdt,
                eth
            );

            // Combine creation code and arguments
            bytes memory deploymentCode = bytes.concat(
                type(LendingPool).creationCode,
                constructorArgs
            );

            // Deploy asynchronously
            address deployedPoolAddress = Nil.asyncDeploy(
                shardId,
                deployer, // refundTo (deployer initiated this)
                deployer, // bounceTo (send error back to deployer if deployment fails)
                0, // feeCredit (default)
                Nil.FORWARD_REMAINING, // forwardKind (default)
                0, // value
                deploymentCode,
                uint256(salt)
            );

            // Register the deployed address
            lendingPoolsByShard[shardId] = deployedPoolAddress;
            isLendingPool[deployedPoolAddress] = true;
            emit PoolRegistered(deployedPoolAddress, shardId); // Emit for consistency
            emit LendingPoolDeployed(deployedPoolAddress, shardId);
        }
    }

    /// @notice Processes a deposit forwarded asynchronously from a LendingPool contract.
    /// @dev Expects the deposited tokens to be included in the transaction (`Nil.txnTokens`).
    /// Increases the collateral balance for the original depositor.
    /// Can only be called by a registered LendingPool.
    /// @param depositor The address of the original user making the deposit.
    function handleDeposit(
        address depositor
    ) public payable onlyRegisteredLendingPool {
        Nil.Token[] memory tokens = Nil.txnTokens();
        TokenId token = tokens[0].id;
        uint256 amount = tokens[0].amount;

        collateralBalances[depositor][token] += amount;
        emit DepositHandled(depositor, token, amount);
    }

    /// @notice Processes a borrow request forwarded asynchronously from a LendingPool contract.
    /// @dev Checks liquidity, verifies collateral, records the loan, and sends the borrowed tokens.
    /// Can only be called by a registered LendingPool.
    /// @param borrower The address of the user borrowing.
    /// @param amount The amount to borrow.
    /// @param borrowToken The token to borrow.
    /// @param requiredCollateral The minimum collateral value required (pre-calculated by LendingPool).
    /// @param collateralToken The token used as collateral.
    function handleBorrowRequest(
        address borrower,
        uint256 amount,
        TokenId borrowToken,
        uint256 requiredCollateral,
        TokenId collateralToken
    ) public onlyRegisteredLendingPool {
        // Check internal liquidity (this contract's balance)
        if (Nil.tokenBalance(address(this), borrowToken) < amount) {
            revert InsufficientLiquidity();
        }

        // Check user's collateral balance stored here
        if (
            collateralBalances[borrower][collateralToken] < requiredCollateral
        ) {
            revert InsufficientCollateral();
        }

        // Record the loan
        loans[borrower] = Loan(amount, borrowToken);

        // Send the borrowed tokens directly to the borrower from this contract's funds
        sendTokenInternal(borrower, borrowToken, amount);

        emit BorrowProcessed(borrower, borrowToken, amount);
    }

    /// @notice Processes a repayment forwarded asynchronously from a LendingPool contract.
    /// @dev Expects the repayment tokens to be included in the transaction (`Nil.txnTokens`).
    /// Verifies the repayment amount against the required amount (principal + interest),
    /// clears the loan record, and returns the collateral to the borrower.
    /// Can only be called by a registered LendingPool.
    /// @param borrower The address of the user repaying.
    /// @param collateralToken The token used as collateral for the loan being repaid.
    /// @param requiredRepaymentAmount The total amount (principal + interest) required (pre-calculated by LendingPool).
    function processRepayment(
        address borrower,
        TokenId collateralToken,
        uint256 requiredRepaymentAmount
    ) public payable onlyRegisteredLendingPool {
        Nil.Token[] memory tokens = Nil.txnTokens();
        TokenId repaidToken = tokens[0].id;
        uint256 sentAmount = tokens[0].amount;

        Loan memory loan = loans[borrower];

        // Check if there is an active loan and the correct token is being repaid
        if (loan.amount == 0 || loan.token != repaidToken) {
            revert NoActiveLoan();
        }

        // Ensure sufficient funds were sent for principal + interest
        if (sentAmount < requiredRepaymentAmount) {
            revert RepaymentInsufficient();
        }

        // Clear the loan record
        delete loans[borrower];
        emit RepaymentProcessed(borrower, repaidToken, loan.amount);

        // Handle collateral release
        uint256 collateralAmount = collateralBalances[borrower][
            collateralToken
        ];
        if (collateralAmount > 0) {
            delete collateralBalances[borrower][collateralToken];
            sendTokenInternal(borrower, collateralToken, collateralAmount);
            emit CollateralReturned(
                borrower,
                collateralToken,
                collateralAmount
            );
        }
    }

    /// @notice Fetches a user's collateral balance for a specific token.
    /// @param user The address of the user.
    /// @param token The token type.
    /// @return uint256 The collateral amount.
    function getCollateralBalance(
        address user,
        TokenId token
    ) public view returns (uint256) {
        return collateralBalances[user][token];
    }

    /// @notice Retrieves a user's active loan details.
    /// @param user The address of the user.
    /// @return amount_ The loan principal amount (0 if no active loan).
    /// @return token_ The token type used for the loan (address(0) if no active loan).
    function getLoanDetails(
        address user
    ) public view returns (uint256 amount_, TokenId token_) {
        return (loans[user].amount, loans[user].token);
    }
}
