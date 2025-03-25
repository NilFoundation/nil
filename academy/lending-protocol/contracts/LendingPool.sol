// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

/// @title LendingPool
/// @dev The LendingPool contract facilitates lending and borrowing of tokens and handles collateral management.
/// It interacts with other contracts such as GlobalLedger, InterestManager, and Oracle for tracking deposits, calculating interest, and fetching token prices.
contract LendingPool is NilBase, NilTokenBase {
    address public deployer;

    address public globalLedger;
    address public interestManager;
    address public oracle;
    TokenId public usdt;
    TokenId public eth;

    //Errors
    error OnlyDeployer();
    error InvalidToken();
    error InvalidCaller();
    error InvalidShardId();
    error ExcessTransfer();
    error InsufficientFunds(string message);
    error InvalidPoolAddress();
    error PoolAlreadyRegistered();
    error InvalidTransferAmount();
    error InsufficientCollateral();
    error FailedToRetrieveCollateral();
    error CrossShardCallFailed(string message);

    /// @dev Mapping to track lending pools on different shards
    mapping(uint256 => address) public lendingPoolsByShard;
    address[] public lendingPools;
    uint256 public lendingPoolCount;

    /// @dev Mapping to track active borrow requests to prevent multiple handling
    mapping(bytes32 => bool) public activeBorrowRequests;

    /// @dev Modifier to ensure only the deployer can call certain functions
    modifier onlyDeployer() {
        if (msg.sender != deployer) revert OnlyDeployer();
        _;
    }

    /// @notice Constructor to initialize the LendingPool contract with addresses for dependencies.
    /// @dev Sets the contract addresses for GlobalLedger, InterestManager, Oracle, USDT, and ETH tokens.
    /// @param _globalLedger The address of the GlobalLedger contract.
    /// @param _interestManager The address of the InterestManager contract.
    /// @param _oracle The address of the Oracle contract.
    /// @param _usdt The TokenId for USDT.
    /// @param _eth The TokenId for ETH.
    constructor(
        address _globalLedger,
        address _interestManager,
        address _oracle,
        TokenId _usdt,
        TokenId _eth
    ) {
        globalLedger = _globalLedger;
        interestManager = _interestManager;
        oracle = _oracle;
        usdt = _usdt;
        eth = _eth;

        deployer = msg.sender;
    }

    /// @notice Register a new lending pool on a different shard
    /// @dev Allows registration of other lending pools for cross-shard borrowing
    /// @param poolAddress The address of the lending pool to register
    function registerLendingPool(address poolAddress) external onlyDeployer {
        if (poolAddress == address(0)) revert InvalidPoolAddress();

        uint256 shardId = Nil.getShardId(poolAddress);
        if (lendingPoolsByShard[shardId] != address(0))
            revert PoolAlreadyRegistered();

        lendingPoolsByShard[shardId] = poolAddress;
        lendingPools.push(poolAddress);
        lendingPoolCount++;
    }

    /// @notice Check liquidity across all registered lending pools
    /// @dev Queries each registered lending pool for available liquidity
    /// @param token The token to check liquidity for
    /// @param amount The amount needed
    /// @param requestId Unique identifier for this borrow request
    function checkCrossShardLiquidity(
        TokenId token,
        uint256 amount,
        bytes32 requestId,
        address borrower
    ) internal {
        uint256 currentShardId = Nil.getShardId(address(this));

        if (lendingPoolCount > 0) {
            for (uint256 i = 0; i < lendingPools.length; i++) {
                address poolAddress = lendingPools[i];
                uint256 poolShardId = Nil.getShardId(poolAddress);

                // Skip if the pool is on the same shard
                if (poolShardId == currentShardId) {
                    continue;
                }

                // Query the other pool's token balance
                bytes memory callData = abi.encodeWithSignature(
                    "getTokenBalance(address)",
                    token
                );

                bytes memory context = abi.encodeWithSelector(
                    this.handleLiquidityCheck.selector,
                    token,
                    amount,
                    poolAddress,
                    poolShardId,
                    requestId,
                    borrower
                );

                Nil.sendRequest(poolAddress, 0, 6_000_000, context, callData);
            }
        }
    }

    /// @notice Handle the response from liquidity check
    /// @dev If liquidity is found on another shard, initiate the cross-shard transfer
    function handleLiquidityCheck(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        if (!success) revert CrossShardCallFailed("Liquidity check failed");

        (
            TokenId token,
            uint256 amount,
            address sourcePool,
            ,
            bytes32 requestId,
            address borrower
        ) = abi.decode(
                context,
                (TokenId, uint256, address, uint256, bytes32, address)
            );

        uint256 availableLiquidity = abi.decode(returnData, (uint256));

        // Only process if this request hasn't been fulfilled yet
        if (!activeBorrowRequests[requestId] && availableLiquidity >= amount) {
            // Mark this request as active to prevent other shards from processing it
            activeBorrowRequests[requestId] = true;

            // Continue with the normal borrow process first
            TokenId collateralToken = (token == usdt) ? eth : usdt;

            bytes memory callData = abi.encodeWithSignature(
                "getPrice(address)",
                token
            );

            // Pass the source pool information in the context for later use
            bytes memory priceContext = abi.encodeWithSelector(
                this.processLoan.selector,
                borrower,
                amount,
                token,
                collateralToken,
                requestId,
                sourcePool
            );

            Nil.sendRequest(oracle, 0, 9_000_000, priceContext, callData);
        }
    }

    /// @notice Transfer liquidity to another shard
    /// @dev Called by other shards to request liquidity,
    /// @dev loan are to be sent directly to the borrower therefore we need to record the loan first
    function transferCrossShardLiquidity(
        address recipient,
        TokenId token,
        uint256 amount
    ) public {
        // Verify we have enough liquidity
        if (Nil.tokenBalance(address(this), token) < amount) {
            revert InsufficientFunds("Insufficient liquidity");
        }

        /// @notice Record the loan in GlobalLedger
        /// @dev The loan details are recorded in the GlobalLedger contract.
        bytes memory recordLoanCallData = abi.encodeWithSignature(
            "recordLoan(address,address,uint256)",
            recipient,
            token,
            amount
        );

        // First record the loan
        Nil.asyncCall(globalLedger, address(this), 0, recordLoanCallData);

        //then Transfer the tokens to the requesting pool
        sendTokenInternal(recipient, token, amount);
    }

    /// @notice Get the balance of a specific token in the lending pool
    /// @dev Used by other lending pools to check available liquidity
    /// @param token The token to check balance for
    /// @return uint256 The balance of the token
    function getTokenBalance(TokenId token) public view returns (uint256) {
        return Nil.tokenBalance(address(this), token);
    }

    /// @notice Deposit function to deposit tokens into the lending pool.
    /// @dev The deposited tokens are recorded in the GlobalLedger via an asynchronous call.
    function deposit() public payable {
        /// Retrieve the tokens being sent in the transaction
        Nil.Token[] memory tokens = Nil.txnTokens();

        /// @notice Encoding the call to the GlobalLedger to record the deposit
        /// @dev The deposit details (user address, token type, and amount) are encoded for GlobalLedger.
        /// @param callData The encoded call data for recording the deposit in GlobalLedger.
        bytes memory callData = abi.encodeWithSignature(
            "recordDeposit(address,address,uint256)",
            msg.sender,
            tokens[0].id, // The token being deposited (usdt or eth)
            tokens[0].amount // The amount of the token being deposited
        );

        /// @notice Making an asynchronous call to the GlobalLedger to record the deposit
        /// @dev This ensures that the user's deposit is recorded in GlobalLedger asynchronously.
        Nil.asyncCall(globalLedger, address(this), 0, callData);
    }

    /// @notice Borrow function allows a user to borrow tokens (either USDT or ETH).
    /// @dev Ensures sufficient liquidity, checks collateral, and processes the loan after fetching the price from the Oracle.
    /// @param amount The amount of the token to borrow.
    /// @param borrowToken The token the user wants to borrow (either USDT or ETH).
    function borrow(uint256 amount, TokenId borrowToken) public payable {
        /// @notice Ensure the token being borrowed is either USDT or ETH
        /// @dev Prevents invalid token types from being borrowed.
        if (borrowToken != usdt && borrowToken != eth) revert InvalidToken();

        /// @notice Check local liquidity first
        /// @dev If local pool has sufficient liquidity, process the borrow locally

        // Create a unique request ID at the start of the borrow process
        bytes32 requestId = keccak256(
            abi.encodePacked(block.timestamp, msg.sender, borrowToken, amount)
        );

        // Check local liquidity first
        if (Nil.tokenBalance(address(this), borrowToken) >= amount) {
            /// @notice Determine which collateral token will be used (opposite of the borrow token)
            /// @dev Identifies the collateral token by comparing the borrow token.
            TokenId collateralToken = (borrowToken == usdt) ? eth : usdt;

            /// @notice Prepare a call to the Oracle to get the price of the borrow token
            /// @dev The price of the borrow token is fetched from the Oracle to calculate collateral.
            /// @param callData The encoded data to fetch the price from the Oracle.
            bytes memory callData = abi.encodeWithSignature(
                "getPrice(address)",
                borrowToken
            );

            /// @notice Encoding the context to process the loan after the price is fetched
            /// @dev The context contains the borrower's details, loan amount, borrow token, and collateral token.
            bytes memory context = abi.encodeWithSelector(
                this.processLoan.selector,
                msg.sender,
                amount,
                borrowToken,
                collateralToken,
                requestId,
                address(this)
            );

            /// @notice Send a request to the Oracle to get the price of the borrow token.
            /// @dev This request is processed with a fee for the transaction, allowing the system to fetch the token price.
            Nil.sendRequest(oracle, 0, 9_000_000, context, callData);
        } else {
            /// @notice If local liquidity is insufficient, check cross-shard liquidity
            /// @dev Queries other lending pools across shards for available liquidity
            checkCrossShardLiquidity(
                borrowToken,
                amount,
                requestId,
                msg.sender
            );
        }
    }

    /// @notice Callback function to process the loan after the price data is retrieved from Oracle.
    /// @dev Ensures that the borrower has enough collateral, calculates the loan value, and initiates loan processing.
    /// @param success Indicates if the Oracle call was successful.
    /// @param returnData The price data returned from the Oracle.
    /// @param context The context data containing borrower details, loan amount, and collateral token.
    function processLoan(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        /// @notice Ensure the Oracle call was successful
        /// @dev Verifies that the price data was successfully retrieved from the Oracle.
        if (!success) revert CrossShardCallFailed("Oracle call failed");

        /// @notice Decode the context to extract borrower details, loan amount, and collateral token
        /// @dev Decodes the context passed from the borrow function to retrieve necessary data.
        (
            address borrower,
            uint256 amount,
            TokenId borrowToken,
            TokenId collateralToken,
            bytes32 requestId,
            address sourcePool
        ) = abi.decode(
                context,
                (address, uint256, TokenId, TokenId, bytes32, address)
            );

        /// @notice Decode the price data returned from the Oracle
        /// @dev The returned price data is used to calculate the loan value in USD.
        uint256 borrowTokenPrice = abi.decode(returnData, (uint256));
        /// @notice Calculate the loan value in USD
        /// @dev Multiplies the amount by the borrow token price to get the loan value in USD.
        uint256 loanValueInUSD = amount * borrowTokenPrice;
        /// @notice Calculate the required collateral (120% of the loan value)
        /// @dev The collateral is calculated as 120% of the loan value to mitigate risk.
        uint256 requiredCollateral = (loanValueInUSD * 120) / 100;

        /// @notice Prepare a call to GlobalLedger to check the user's collateral balance
        /// @dev Fetches the collateral balance from the GlobalLedger contract to ensure sufficient collateral.
        bytes memory ledgerCallData = abi.encodeWithSignature(
            "getDeposit(address,address)",
            borrower,
            collateralToken
        );

        /// @notice Encoding the context to finalize the loan once the collateral is validated
        /// @dev Once the collateral balance is validated, the loan is finalized and processed.
        bytes memory ledgerContext = abi.encodeWithSelector(
            this.finalizeLoan.selector,
            borrower,
            amount,
            borrowToken,
            requiredCollateral,
            requestId,
            sourcePool
        );

        /// @notice Send request to GlobalLedger to get the user's collateral
        /// @dev The fee for this request is retained for processing the collateral validation response.
        Nil.sendRequest(
            globalLedger,
            0,
            6_000_000,
            ledgerContext,
            ledgerCallData
        );
    }

    /// @notice Finalize the loan by ensuring sufficient collateral and recording the loan in GlobalLedger.
    /// @dev Verifies that the user has enough collateral, processes the loan, and sends the borrowed tokens to the borrower.
    /// @param success Indicates if the collateral check was successful.
    /// @param returnData The collateral balance returned from the GlobalLedger.
    /// @param context The context containing loan details.
    function finalizeLoan(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        /// @notice Ensure the collateral check was successful
        /// @dev Verifies the collateral validation result from GlobalLedger.
        if (!success) revert CrossShardCallFailed("Collateral check failed");

        /// @notice Decode the context to extract loan details
        /// @dev Decodes the context passed from the processLoan function to retrieve loan data.
        (
            address borrower,
            uint256 amount,
            TokenId borrowToken,
            uint256 requiredCollateral,
            ,
            address sourcePool
        ) = abi.decode(
                context,
                (address, uint256, TokenId, uint256, bytes32, address)
            );

        /// @notice Decode the user's collateral balance from GlobalLedger
        /// @dev Retrieves the user's collateral balance from the GlobalLedger to compare it with the required collateral.
        uint256 userCollateral = abi.decode(returnData, (uint256));

        /// @notice Check if the user has enough collateral to cover the loan
        /// @dev Ensures the borrower has sufficient collateral before proceeding with the loan.
        if (userCollateral < requiredCollateral)
            revert InsufficientFunds("Insufficient collateral");

        if (sourcePool != address(this)) {
            /// @notice Request the transfer from the source pool to the borrower
            // Then request the transfer from source pool to borrower
            bytes memory transferCallData = abi.encodeWithSignature(
                "transferCrossShardLiquidity(address,address,uint256)",
                borrower,
                borrowToken,
                amount
            );

            Nil.asyncCall(sourcePool, address(this), 0, transferCallData);
        } else {
            transferCrossShardLiquidity(borrower, borrowToken, amount);
        }
    }

    /// @notice Repay loan function called by the borrower to repay their loan.
    /// @dev Initiates the repayment process by retrieving the loan details from GlobalLedger.
    function repayLoan() public payable {
        /// @notice Retrieve the tokens being sent in the transaction
        /// @dev Retrieves the tokens involved in the repayment.
        Nil.Token[] memory tokens = Nil.txnTokens();

        /// @notice Prepare to query the loan details from GlobalLedger
        /// @dev Fetches the loan details of the borrower to proceed with repayment.
        bytes memory callData = abi.encodeWithSignature(
            "getLoanDetails(address)",
            msg.sender
        );

        /// @notice Encoding the context to handle repayment after loan details are fetched
        /// @dev Once the loan details are retrieved, the repayment amount is processed.
        bytes memory context = abi.encodeWithSelector(
            this.handleRepayment.selector,
            msg.sender,
            tokens[0].amount
        );

        /// @notice Send request to GlobalLedger to fetch loan details
        /// @dev Retrieves the borrower's loan details before proceeding with the repayment.
        Nil.sendRequest(globalLedger, 0, 11_000_000, context, callData);
    }

    /// @notice Handle the loan repayment, calculate the interest, and update GlobalLedger.
    /// @dev Calculates the total repayment (principal + interest) and updates the loan status in GlobalLedger.
    /// @param success Indicates if the loan details retrieval was successful.
    /// @param returnData The loan details returned from the GlobalLedger.
    /// @param context The context containing borrower and repayment details.
    function handleRepayment(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        /// @notice Ensure the GlobalLedger call was successful
        /// @dev Verifies that the loan details were successfully retrieved from the GlobalLedger.
        if (!success) revert CrossShardCallFailed("Ledger call failed");

        /// @notice Decode context and loan details
        /// @dev Decodes the context and the return data to retrieve the borrower's loan details.
        (address borrower, uint256 sentAmount) = abi.decode(
            context,
            (address, uint256)
        );
        (uint256 amount, TokenId token) = abi.decode(
            returnData,
            (uint256, TokenId)
        );

        /// @notice Ensure the borrower has an active loan
        /// @dev Ensures the borrower has an outstanding loan before proceeding with repayment.
        if (amount <= 0) revert InsufficientFunds("No active loan");

        /// @notice Request the interest rate from the InterestManager
        /// @dev Fetches the current interest rate for the loan from the InterestManager contract.
        bytes memory interestCallData = abi.encodeWithSignature(
            "getInterestRate()"
        );
        bytes memory interestContext = abi.encodeWithSelector(
            this.processRepayment.selector,
            borrower,
            amount,
            token,
            sentAmount
        );

        /// @notice Send request to InterestManager to fetch interest rate
        /// @dev This request fetches the interest rate that will be used to calculate the total repayment.
        Nil.sendRequest(
            interestManager,
            0,
            8_000_000,
            interestContext,
            interestCallData
        );
    }

    /// @notice Process the repayment, calculate the total repayment including interest.
    /// @dev Finalizes the loan repayment, ensuring the borrower has sent sufficient funds.
    /// @param success Indicates if the interest rate call was successful.
    /// @param returnData The interest rate returned from the InterestManager.
    /// @param context The context containing repayment details.
    function processRepayment(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        /// @notice Ensure the interest rate call was successful
        /// @dev Verifies that the interest rate retrieval was successful.
        if (!success) revert CrossShardCallFailed("Interest rate call failed");

        /// @notice Decode the repayment details and the interest rate
        /// @dev Decodes the repayment context and retrieves the interest rate for loan repayment.
        (
            address borrower,
            uint256 amount,
            TokenId token,
            uint256 sentAmount
        ) = abi.decode(context, (address, uint256, TokenId, uint256));

        /// @notice Decode the interest rate from the response
        /// @dev Decodes the interest rate received from the InterestManager contract.
        uint256 interestRate = abi.decode(returnData, (uint256));
        /// @notice Calculate the total repayment amount (principal + interest)
        /// @dev Adds the interest to the principal to calculate the total repayment due.
        uint256 totalRepayment = amount + ((amount * interestRate) / 100);

        /// @notice Ensure the borrower has sent sufficient funds for the repayment
        /// @dev Verifies that the borrower has provided enough funds to repay the loan in full.
        if (sentAmount < totalRepayment)
            revert InsufficientFunds("Insufficient repayment amount");

        /// @notice Clear the loan and release collateral
        /// @dev Marks the loan as repaid and releases any associated collateral back to the borrower.
        bytes memory clearLoanCallData = abi.encodeWithSignature(
            "recordLoan(address,address,uint256)",
            borrower,
            token,
            0 // Mark the loan as repaid
        );
        bytes memory releaseCollateralContext = abi.encodeWithSelector(
            this.releaseCollateral.selector,
            borrower,
            token
        );

        /// @notice Send request to GlobalLedger to update the loan status
        /// @dev Updates the loan status to indicate repayment completion in the GlobalLedger.
        Nil.sendRequest(
            globalLedger,
            0,
            6_000_000,
            releaseCollateralContext,
            clearLoanCallData
        );
    }

    /// @notice Release the collateral after the loan is repaid.
    /// @dev Sends the collateral back to the borrower after confirming the loan is fully repaid.
    /// @param success Indicates if the loan clearing was successful.
    /// @param returnData The collateral data returned from the GlobalLedger.
    /// @param context The context containing borrower and collateral token.
    function releaseCollateral(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        /// @notice Ensure the loan clearing was successful
        /// @dev Verifies the result of clearing the loan in the GlobalLedger.
        if (!success) revert CrossShardCallFailed("Loan clearing failed");

        /// @notice Silence unused variable warning
        /// @dev A placeholder for unused variables to avoid compiler warnings.
        returnData;

        /// @notice Decode context for borrower and collateral token
        /// @dev Decodes the context passed from the loan clearing function to retrieve the borrower's details.
        (address borrower, TokenId borrowToken) = abi.decode(
            context,
            (address, TokenId)
        );

        /// @notice Determine the collateral token (opposite of borrow token)
        /// @dev Identifies the token being used as collateral based on the borrow token.
        TokenId collateralToken = (borrowToken == usdt) ? eth : usdt;

        /// @notice Request collateral amount from GlobalLedger
        /// @dev Retrieves the amount of collateral associated with the borrower from the GlobalLedger.
        bytes memory getCollateralCallData = abi.encodeWithSignature(
            "getDeposit(address,address)",
            borrower,
            collateralToken
        );

        /// @notice Context to send collateral to the borrower
        /// @dev After confirming the collateral balance, it is returned to the borrower.
        bytes memory sendCollateralContext = abi.encodeWithSelector(
            this.sendCollateral.selector,
            borrower,
            collateralToken
        );

        /// @notice Send request to GlobalLedger to retrieve the collateral
        /// @dev This request ensures that the correct collateral is available for release.
        Nil.sendRequest(
            globalLedger,
            0,
            3_50_000,
            sendCollateralContext,
            getCollateralCallData
        );
    }

    /// @notice Send the collateral back to the borrower.
    /// @dev Ensures there is enough collateral to release and then sends the funds back to the borrower.
    /// @param success Indicates if the collateral retrieval was successful.
    /// @param returnData The amount of collateral available.
    /// @param context The context containing borrower and collateral token.
    function sendCollateral(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        /// @notice Ensure the collateral retrieval was successful
        /// @dev Verifies that the request to retrieve the collateral was successful.
        if (!success)
            revert CrossShardCallFailed("Collateral retrieval failed");

        /// @notice Decode the collateral details
        /// @dev Decodes the context passed from the releaseCollateral function to retrieve collateral details.
        (address borrower, TokenId collateralToken) = abi.decode(
            context,
            (address, TokenId)
        );
        uint256 collateralAmount = abi.decode(returnData, (uint256));

        /// @notice Ensure there's collateral to release
        /// @dev Verifies that there is enough collateral to be released.
        if (collateralAmount <= 0) revert FailedToRetrieveCollateral();

        /// @notice Ensure sufficient balance in the LendingPool to send collateral
        /// @dev Verifies that the LendingPool has enough collateral to send to the borrower.
        if (Nil.tokenBalance(address(this), collateralToken) < collateralAmount)
            revert InsufficientFunds("Insufficient collateral balance");

        /// @notice Send the collateral tokens to the borrower
        /// @dev Executes the transfer of collateral tokens back to the borrower.
        sendTokenInternal(borrower, collateralToken, collateralAmount);
    }
}
