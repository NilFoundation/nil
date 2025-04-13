// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

/// @title LendingPool (Shard Entry Point)
/// @dev The LendingPool contract acts as a user interface on each shard.
/// It forwards requests to the CentralLedger and interacts with Oracle and InterestManager.
contract LendingPool is NilBase, NilTokenBase {
    // Core contract addresses
    address public centralLedger;
    address public interestManager;
    address public oracle;
    TokenId public usdt;
    TokenId public eth;

    // Errors
    error InvalidToken();
    error InsufficientFunds(string message);
    error CrossShardCallFailed(string message);

    // Events
    event DepositInitiated(address indexed user, TokenId token, uint256 amount);
    event LoanRequested(
        address indexed borrower,
        uint256 amount,
        TokenId borrowToken,
        TokenId collateralToken
    );
    event RepaymentInitiated(
        address indexed borrower,
        TokenId token,
        uint256 amount
    );

    /// @notice Constructor to initialize the LendingPool shard contract.
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

    /// @notice Deposit function to deposit tokens into the protocol via the CentralLedger.
    function deposit() public payable {
        Nil.Token[] memory tokens = Nil.txnTokens();
        require(tokens.length == 1, "Only one token type per deposit");

        bytes memory callData = abi.encodeWithSelector(
            bytes4(keccak256("handleDeposit(address)")),
            msg.sender
        );

        // Use Nil.asyncCallWithTokens, providing necessary arguments
        Nil.asyncCallWithTokens(
            centralLedger,
            address(0), // refundTo (default)
            address(this), // bounceTo
            0, // feeCredit (default)
            Nil.FORWARD_REMAINING, // forwardKind (default)
            0, // value
            tokens, // The deposit tokens
            callData
        );

        emit DepositInitiated(msg.sender, tokens[0].id, tokens[0].amount);
    }

    /// @notice Initiates a borrow request.
    function borrow(uint256 amount, TokenId borrowToken) public payable {
        if (borrowToken != usdt && borrowToken != eth) revert InvalidToken();

        TokenId collateralToken = (borrowToken == usdt) ? eth : usdt;

        bytes memory oracleCallData = abi.encodeWithSignature(
            "getPrice(address)",
            borrowToken
        );

        bytes memory context = abi.encodeWithSelector(
            this.processOracleResponse.selector,
            msg.sender,
            amount,
            borrowToken,
            collateralToken
        );

        // Use Nil.sendRequest
        Nil.sendRequest(oracle, 0, 11_000_000, context, oracleCallData);

        emit LoanRequested(msg.sender, amount, borrowToken, collateralToken);
    }

    /// @notice Callback after Oracle returns the price for a borrow request.
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

        uint256 loanValueInUSD = amount * borrowTokenPrice;
        uint256 requiredCollateralValue = (loanValueInUSD * 120) / 100;

        bytes memory ledgerCallData = abi.encodeWithSignature(
            "getCollateralBalance(address,address)",
            borrower,
            collateralToken
        );

        bytes memory ledgerContext = abi.encodeWithSelector(
            this.finalizeBorrow.selector,
            borrower,
            amount,
            borrowToken,
            requiredCollateralValue,
            collateralToken
        );

        // Use Nil.sendRequest
        Nil.sendRequest(
            centralLedger,
            0,
            8_000_000,
            ledgerContext,
            ledgerCallData
        );
    }

    /// @notice Callback after CentralLedger returns the user's collateral balance.
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

        uint256 userCollateralValue = abi.decode(returnData, (uint256));

        if (userCollateralValue < requiredCollateralValue)
            revert InsufficientFunds("Insufficient collateral value");

        bytes memory centralLedgerCallData = abi.encodeWithSelector(
            bytes4(
                keccak256(
                    "handleBorrowRequest(address,uint256,address,uint256,address)"
                )
            ),
            borrower,
            amount,
            borrowToken,
            requiredCollateralValue,
            collateralToken
        );

        // Use Nil.asyncCall (no tokens transferred here)
        Nil.asyncCall(centralLedger, address(this), 0, centralLedgerCallData);
    }

    /// @notice Initiates the loan repayment process.
    function repayLoan() public payable {
        Nil.Token[] memory tokens = Nil.txnTokens();
        require(tokens.length == 1, "Only one token type per repayment");
        TokenId repaidToken = tokens[0].id;
        uint256 sentAmount = tokens[0].amount;
        address borrower = msg.sender;

        bytes memory getLoanCallData = abi.encodeWithSignature(
            "getLoanDetails(address)",
            borrower
        );

        bytes memory context = abi.encodeWithSelector(
            this.handleLoanDetailsForRepayment.selector,
            borrower,
            sentAmount,
            repaidToken
        );

        // Use Nil.sendRequest
        Nil.sendRequest(centralLedger, 0, 8_000_000, context, getLoanCallData);

        emit RepaymentInitiated(borrower, repaidToken, sentAmount);
    }

    /// @notice Callback after fetching loan details for repayment.
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

        if (loanAmount == 0) revert InsufficientFunds("No active loan found");
        if (loanToken != repaidToken) revert InvalidToken();

        bytes memory interestCallData = abi.encodeWithSignature(
            "getInterestRate()"
        );

        bytes memory interestContext = abi.encodeWithSelector(
            this.processRepaymentCalculation.selector,
            borrower,
            loanAmount,
            loanToken,
            sentAmount
        );

        // Use Nil.sendRequest
        Nil.sendRequest(
            interestManager,
            0,
            8_000_000,
            interestContext,
            interestCallData
        );
    }

    /// @notice Callback after fetching the interest rate.
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

        uint256 interestRate = abi.decode(returnData, (uint256));

        uint256 interestAmount = (loanAmount * interestRate) / 100;
        uint256 totalRepayment = loanAmount + interestAmount;

        if (sentAmount < totalRepayment)
            revert InsufficientFunds("Insufficient amount sent for repayment");

        TokenId collateralToken = (loanToken == usdt) ? eth : usdt;

        bytes memory processRepaymentCallData = abi.encodeWithSelector(
            bytes4(keccak256("processRepayment(address,address,uint256)")),
            borrower,
            collateralToken,
            totalRepayment
        );

        Nil.Token[] memory tokensToForward = new Nil.Token[](1);
        tokensToForward[0] = Nil.Token(loanToken, sentAmount);

        // Use Nil.asyncCallWithTokens, providing necessary arguments
        Nil.asyncCallWithTokens(
            centralLedger,
            address(0), // refundTo (default)
            address(this), // bounceTo
            0, // feeCredit (default)
            Nil.FORWARD_REMAINING, // forwardKind (default)
            0, // value
            tokensToForward, // The repayment tokens
            processRepaymentCallData
        );
    }
}
