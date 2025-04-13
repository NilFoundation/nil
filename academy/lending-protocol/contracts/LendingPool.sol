// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

/// @title LendingPool (Shard Entry Point)
/// @author Your Name/Organization
/// @notice Acts as a user-facing entry point on a specific shard for the lending protocol.
/// @dev Handles user interactions (deposit, borrow, repay) and orchestrates asynchronous calls
/// to the CentralLedger, Oracle, and InterestManager contracts to fulfill requests.
/// This contract itself does not hold significant state or liquidity.
contract LendingPool is NilTokenBase {
    // --- State Variables --- //
    /// @notice The address of the central GlobalLedger contract.
    address public centralLedger;
    /// @notice The address of the InterestManager contract.
    address public interestManager;
    /// @notice The address of the Oracle contract.
    address public oracle;
    /// @notice The TokenId for the USDT token.
    TokenId public usdt;
    /// @notice The TokenId for the ETH token.
    TokenId public eth;

    // --- Errors --- //
    /// @dev Reverts if an unsupported or invalid token (not USDT or ETH) is used.
    error InvalidToken();
    /// @dev Reverts if a user provides insufficient funds (e.g., for repayment) or collateral.
    error InsufficientFunds(string message);
    /// @dev Reverts if a required cross-shard call (e.g., to Oracle, InterestManager, CentralLedger) fails.
    error CrossShardCallFailed(string message);

    // --- Events --- //
    /// @notice Emitted when a user initiates a deposit via this pool.
    /// @param user The address of the user initiating the deposit.
    /// @param token The TokenId of the asset being deposited.
    /// @param amount The amount being deposited.
    event DepositInitiated(address indexed user, TokenId token, uint256 amount);
    /// @notice Emitted when a user initiates a borrow request.
    /// @param borrower The address of the user initiating the borrow.
    /// @param amount The amount requested to borrow.
    /// @param borrowToken The TokenId of the asset requested.
    /// @param collateralToken The TokenId of the asset used as collateral.
    event LoanRequested(
        address indexed borrower,
        uint256 amount,
        TokenId borrowToken,
        TokenId collateralToken
    );
    /// @notice Emitted when a user initiates a loan repayment.
    /// @param borrower The address of the user initiating the repayment.
    /// @param token The TokenId of the asset being repaid.
    /// @param amount The amount sent for repayment.
    event RepaymentInitiated(
        address indexed borrower,
        TokenId token,
        uint256 amount
    );

    /// @notice Initializes the LendingPool shard contract.
    /// @param _centralLedger The address of the central GlobalLedger contract.
    /// @param _interestManager The address of the InterestManager contract.
    /// @param _oracle The address of the Oracle contract.
    /// @param _usdt The TokenId for USDT.
    /// @param _eth The TokenId for ETH.
    constructor(
        address _centralLedger,
        address _interestManager,
        address _oracle,
        TokenId _usdt,
        TokenId _eth
    ) {
        centralLedger = _centralLedger;
        interestManager = _interestManager;
        oracle = _oracle;
        usdt = _usdt;
        eth = _eth;
    }

    /// @notice Allows a user to deposit collateral (USDT or ETH) into the protocol.
    /// @dev Forwards the deposit details and tokens to the `CentralLedger` via an asynchronous call.
    /// Requires the user to send exactly one type of token (USDT or ETH) with the transaction.
    function deposit() public payable {
        Nil.Token[] memory tokens = Nil.txnTokens();
        require(tokens.length == 1, "Only one token type per deposit");
        address user = msg.sender;

        bytes memory callData = abi.encodeWithSelector(
            bytes4(keccak256("handleDeposit(address)")),
            user
        );

        // Use Nil.asyncCallWithTokens to forward deposit to CentralLedger
        Nil.asyncCallWithTokens(
            centralLedger,
            user, // refundTo (original user)
            user, // bounceTo (original user)
            0, // feeCredit (default)
            Nil.FORWARD_REMAINING, // forwardKind (default)
            0, // value
            tokens, // The deposit tokens
            callData
        );

        emit DepositInitiated(user, tokens[0].id, tokens[0].amount);
    }

    /// @notice Allows a user to initiate a request to borrow USDT or ETH.
    /// @dev Requires the user to have sufficient collateral deposited in the CentralLedger.
    /// The process involves multiple asynchronous steps:
    /// 1. This function requests the price of the `borrowToken` from the `Oracle`.
    /// 2. The `processOracleResponse` callback calculates required collateral and requests the user's collateral balance from `CentralLedger`.
    /// 3. The `finalizeBorrow` callback verifies collateral and requests `CentralLedger` to execute the borrow.
    /// @param amount The amount of the token to borrow.
    /// @param borrowToken The TokenId of the token to borrow (must be USDT or ETH).
    function borrow(uint256 amount, TokenId borrowToken) public payable {
        if (borrowToken != usdt && borrowToken != eth) revert InvalidToken();

        TokenId collateralToken = (borrowToken == usdt) ? eth : usdt;

        // Step 1: Get Price from Oracle
        bytes memory oracleCallData = abi.encodeWithSignature(
            "getPrice(address)", // Oracle expects address type for TokenId
            borrowToken
        );

        bytes memory context = abi.encodeWithSelector(
            this.processOracleResponse.selector, // Callback function
            msg.sender, // borrower
            amount,
            borrowToken,
            collateralToken
        );

        Nil.sendRequest(oracle, 0, 11_000_000, context, oracleCallData);

        emit LoanRequested(msg.sender, amount, borrowToken, collateralToken);
    }

    /// @notice Callback function triggered after the Oracle returns the borrow token price.
    /// @dev Calculates the required collateral value based on the price and LTV (hardcoded 120% here).
    /// Sends a request to the `CentralLedger` to get the user's current collateral balance.
    /// @param success Boolean indicating if the Oracle call was successful.
    /// @param returnData Encoded price data (uint256) from the Oracle.
    /// @param context Encoded context data passed from the initial `borrow` call.
    function processOracleResponse(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        if (!success) revert CrossShardCallFailed("Oracle price call failed");

        (
            address borrower,
            uint256 amount,
            TokenId borrowToken,
            TokenId collateralToken
        ) = abi.decode(context, (address, uint256, TokenId, TokenId));

        uint256 borrowTokenPrice = abi.decode(returnData, (uint256));

        // Calculate required collateral value (e.g., 120% LTV)
        // Ensure prices are scaled appropriately for calculation
        uint256 loanValueInUSD = amount * borrowTokenPrice;
        uint256 requiredCollateralValue = (loanValueInUSD * 120) / 100;

        // Step 2: Get Collateral Balance from CentralLedger
        bytes memory ledgerCallData = abi.encodeWithSignature(
            "getCollateralBalance(address,address)", // CentralLedger expects address type for TokenId
            borrower,
            collateralToken
        );

        bytes memory ledgerContext = abi.encodeWithSelector(
            this.finalizeBorrow.selector, // Next callback function
            borrower,
            amount,
            borrowToken,
            requiredCollateralValue, // Pass the calculated required collateral value
            collateralToken
        );

        Nil.sendRequest(
            centralLedger,
            0,
            8_000_000,
            ledgerContext,
            ledgerCallData
        );
    }

    /// @notice Callback function triggered after the CentralLedger returns the user's collateral balance.
    /// @dev Verifies if the user's collateral value meets the required amount.
    /// If sufficient, sends an asynchronous call to the `CentralLedger` to execute the borrow
    /// (transfer funds to borrower, record loan state).
    /// @param success Boolean indicating if the CentralLedger call was successful.
    /// @param returnData Encoded collateral balance (uint256) from the CentralLedger.
    /// @param context Encoded context data passed from the `processOracleResponse` call.
    function finalizeBorrow(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        if (!success) revert CrossShardCallFailed("Collateral check failed");

        (
            address borrower,
            uint256 amount,
            TokenId borrowToken,
            uint256 requiredCollateralValue,
            TokenId collateralToken
        ) = abi.decode(context, (address, uint256, TokenId, uint256, TokenId));

        uint256 userCollateralValue = abi.decode(returnData, (uint256)); // Assuming Ledger returns collateral value

        // Check collateral sufficiency
        if (userCollateralValue < requiredCollateralValue) {
            revert InsufficientFunds("Insufficient collateral value");
        }

        // Step 3: Execute Borrow on CentralLedger
        bytes memory centralLedgerCallData = abi.encodeWithSelector(
            bytes4(
                keccak256(
                    "handleBorrowRequest(address,uint256,address,uint256,address)"
                )
            ),
            borrower,
            amount,
            borrowToken,
            requiredCollateralValue, // Pass required value for CentralLedger check
            collateralToken
        );

        // Use Nil.asyncCall with explicit refundTo/bounceTo
        Nil.asyncCall(
            centralLedger,
            borrower, // refundTo (original user)
            borrower, // bounceTo (original user)
            0, // feeCredit (default)
            Nil.FORWARD_REMAINING, // forwardKind (default)
            0, // value
            centralLedgerCallData
        );
    }

    /// @notice Allows a user to initiate repayment of their active loan.
    /// @dev Requires the user to send the repayment token (must match the borrowed token) with the transaction.
    /// The process involves multiple asynchronous steps:
    /// 1. This function requests loan details from the `CentralLedger`.
    /// 2. `handleLoanDetailsForRepayment` callback verifies loan and requests interest rate from `InterestManager`.
    /// 3. `processRepaymentCalculation` callback calculates total repayment and calls `CentralLedger` to process it (clear loan, return collateral).
    function repayLoan() public payable {
        Nil.Token[] memory tokens = Nil.txnTokens();
        require(tokens.length == 1, "Only one token type per repayment");
        TokenId repaidToken = tokens[0].id;
        uint256 sentAmount = tokens[0].amount;
        address borrower = msg.sender;

        // Step 1: Get Loan Details from CentralLedger
        bytes memory getLoanCallData = abi.encodeWithSignature(
            "getLoanDetails(address)",
            borrower
        );

        bytes memory context = abi.encodeWithSelector(
            this.handleLoanDetailsForRepayment.selector, // Callback function
            borrower,
            sentAmount,
            repaidToken // Pass token sent by user for validation
        );

        Nil.sendRequest(centralLedger, 0, 8_000_000, context, getLoanCallData);

        emit RepaymentInitiated(borrower, repaidToken, sentAmount);
    }

    /// @notice Callback function triggered after fetching loan details for repayment.
    /// @dev Verifies that an active loan exists and the user is repaying the correct token.
    /// Sends a request to the `InterestManager` to get the current interest rate.
    /// @param success Boolean indicating if the CentralLedger call was successful.
    /// @param returnData Encoded loan details (uint256 amount, TokenId token) from CentralLedger.
    /// @param context Encoded context data passed from the initial `repayLoan` call.
    function handleLoanDetailsForRepayment(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        if (!success) revert CrossShardCallFailed("Get loan details failed");

        (address borrower, uint256 sentAmount, TokenId repaidToken) = abi
            .decode(context, (address, uint256, TokenId));
        (uint256 loanAmount, TokenId loanToken) = abi.decode(
            returnData,
            (uint256, TokenId)
        );

        // Validate loan existence and correct repayment token
        if (loanAmount == 0) revert InsufficientFunds("No active loan found");
        if (loanToken != repaidToken) revert InvalidToken();

        // Step 2: Get Interest Rate from InterestManager
        bytes memory interestCallData = abi.encodeWithSignature(
            "getInterestRate()"
        );

        bytes memory interestContext = abi.encodeWithSelector(
            this.processRepaymentCalculation.selector, // Next callback function
            borrower,
            loanAmount, // Actual loan amount
            loanToken, // Actual loan token
            sentAmount // Amount user sent
        );

        Nil.sendRequest(
            interestManager,
            0,
            8_000_000,
            interestContext,
            interestCallData
        );
    }

    /// @notice Callback function triggered after fetching the interest rate for repayment.
    /// @dev Calculates the total required repayment amount (principal + interest).
    /// Verifies if the user sent sufficient funds.
    /// Sends an asynchronous call to the `CentralLedger` to process the repayment, forwarding the repayment tokens.
    /// @param success Boolean indicating if the InterestManager call was successful.
    /// @param returnData Encoded interest rate (uint256) from the InterestManager.
    /// @param context Encoded context data passed from the `handleLoanDetailsForRepayment` call.
    function processRepaymentCalculation(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        if (!success) revert CrossShardCallFailed("Interest rate call failed");

        (
            address borrower,
            uint256 loanAmount,
            TokenId loanToken,
            uint256 sentAmount
        ) = abi.decode(context, (address, uint256, TokenId, uint256));

        uint256 interestRate = abi.decode(returnData, (uint256)); // Assume rate is % points (e.g., 5 for 5%)

        // Calculate total repayment required
        uint256 interestAmount = (loanAmount * interestRate) / 100; // Consider precision
        uint256 totalRepayment = loanAmount + interestAmount;

        // Verify user sent enough
        if (sentAmount < totalRepayment) {
            revert InsufficientFunds("Insufficient amount sent for repayment");
        }

        // Determine collateral token associated with the loan
        TokenId collateralToken = (loanToken == usdt) ? eth : usdt;

        // Step 3: Call CentralLedger to Process Repayment
        bytes memory processRepaymentCallData = abi.encodeWithSelector(
            bytes4(keccak256("processRepayment(address,address,uint256)")),
            borrower,
            collateralToken,
            totalRepayment // Tell CentralLedger the amount *required* for accounting
        );

        // Prepare the tokens *actually sent* by the user to be forwarded
        Nil.Token[] memory tokensToForward = new Nil.Token[](1);
        tokensToForward[0] = Nil.Token(loanToken, sentAmount);

        // Use Nil.asyncCallWithTokens to forward repayment to CentralLedger
        Nil.asyncCallWithTokens(
            centralLedger,
            borrower, // refundTo (original user)
            borrower, // bounceTo (original user)
            0, // feeCredit (default)
            Nil.FORWARD_REMAINING, // forwardKind (default)
            0, // value
            tokensToForward, // The repayment tokens sent by the user
            processRepaymentCallData
        );
    }
}
