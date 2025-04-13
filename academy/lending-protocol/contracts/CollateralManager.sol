// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

/// @title GlobalLedger (Centralized)
/// @dev The GlobalLedger contract is responsible for tracking user collateral and loans,
/// and holding all protocol liquidity in the lending protocol.
contract GlobalLedger is NilTokenBase {
    address public deployer;
    address public interestManager;
    address public oracle;
    TokenId public usdt;
    TokenId public eth;

    //Errors
    error OnlyDeployer();
    error InvalidPoolAddress();
    error PoolAlreadyRegistered();
    error UnauthorizedCaller();
    error InsufficientCollateral();
    error InsufficientLiquidity();
    error RepaymentInsufficient();
    error NoActiveLoan();
    error CrossShardCallFailed(string message);

    // Events
    event PoolRegistered(address indexed poolAddress, uint256 shardId);
    event DepositHandled(address indexed user, TokenId token, uint256 amount);
    event BorrowProcessed(
        address indexed borrower,
        TokenId token,
        uint256 amount
    );
    event RepaymentProcessed(
        address indexed borrower,
        TokenId token,
        uint256 amount
    );
    event CollateralReturned(
        address indexed borrower,
        TokenId token,
        uint256 amount
    );

    /// @dev Mapping to track lending pools on different shards
    mapping(uint256 => address) public lendingPoolsByShard;
    /// @dev Quick lookup for registered lending pools
    mapping(address => bool) public isLendingPool;

    /// @dev Mapping of user addresses to their collateral balances (token -> amount). Renamed from deposits.
    mapping(address => mapping(TokenId => uint256)) public collateralBalances;

    /// @dev Mapping of user addresses to their loans (loan amount and loan token).
    mapping(address => Loan) public loans;

    /// @dev Struct to store loan details: amount and the token type.
    struct Loan {
        uint256 amount;
        TokenId token;
    }

    /// @dev Modifier to ensure only the deployer can call certain functions
    modifier onlyDeployer() {
        if (msg.sender != deployer) revert OnlyDeployer();
        _;
    }

    /// @dev Modifier to ensure only a registered LendingPool contract can call
    modifier onlyRegisteredLendingPool() {
        if (!isLendingPool[msg.sender]) revert UnauthorizedCaller();
        _;
    }

    /// @notice Constructor to initialize the GlobalLedger contract.
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

    /// @notice Register a new lending pool
    /// @param poolAddress The address of the lending pool to register
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

    /// @notice Handles a user's deposit forwarded from a LendingPool.
    /// @dev Increases the collateral balance for the user for the specified token. This contract receives the tokens.
    /// @param depositor The address of the original user making the deposit.
    function handleDeposit(
        address depositor
    ) public payable onlyRegisteredLendingPool {
        Nil.Token[] memory tokens = Nil.txnTokens();
        // Assuming only one token type per deposit transaction for simplicity in this example
        require(tokens.length == 1, "Only one token type per deposit");
        TokenId token = tokens[0].id;
        uint256 amount = tokens[0].amount;

        // Increment collateral balance for the depositor
        collateralBalances[depositor][token] += amount;
        emit DepositHandled(depositor, token, amount);
    }

    /// @notice Handles a borrow request forwarded from a LendingPool.
    /// @dev Checks liquidity and collateral, records the loan, and sends funds to the borrower.
    /// @param borrower The address of the user borrowing.
    /// @param amount The amount to borrow.
    /// @param borrowToken The token to borrow.
    /// @param requiredCollateral The minimum collateral value required (already calculated by LendingPool).
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

    /// @notice Processes a repayment forwarded from a LendingPool.
    /// @dev Verifies repayment amount, clears the loan, and returns collateral. This contract receives the repayment tokens.
    /// @param borrower The address of the user repaying.
    /// @param collateralToken The token used as collateral for the loan being repaid.
    /// @param requiredRepaymentAmount The total amount (principal + interest) required, calculated by LendingPool.
    function processRepayment(
        address borrower,
        TokenId collateralToken,
        uint256 requiredRepaymentAmount
    ) public payable onlyRegisteredLendingPool {
        Nil.Token[] memory tokens = Nil.txnTokens();
        // Assuming only one token type per repayment transaction for simplicity
        require(tokens.length == 1, "Only one token type per repayment");
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
        emit RepaymentProcessed(borrower, repaidToken, loan.amount); // Emit principal amount repaid

        // Handle collateral release
        uint256 collateralAmount = collateralBalances[borrower][
            collateralToken
        ];
        if (collateralAmount > 0) {
            delete collateralBalances[borrower][collateralToken]; // Clear collateral record first

            // Return collateral tokens directly to the borrower from this contract's funds
            sendTokenInternal(borrower, collateralToken, collateralAmount);
            emit CollateralReturned(
                borrower,
                collateralToken,
                collateralAmount
            );
        }
    }

    /// @notice Fetches a user's collateral balance for a specific token. (Renamed from getDeposit)
    /// @param user The address of the user.
    /// @param token The token type.
    /// @return uint256 The collateral amount.
    function getCollateralBalance(
        address user,
        TokenId token
    ) public view returns (uint256) {
        return collateralBalances[user][token];
    }

    /// @notice Retrieves a user's loan details.
    /// @dev Returns the loan amount and the token used for the loan.
    /// @param user The address of the user whose loan details are being fetched.
    /// @return uint256 The loan amount.
    /// @return TokenId The token type used for the loan.
    function getLoanDetails(
        address user
    ) public view returns (uint256, TokenId) {
        return (loans[user].amount, loans[user].token);
    }
}
