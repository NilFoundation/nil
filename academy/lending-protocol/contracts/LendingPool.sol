// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

/// @title LendingPool
/// @dev Manages deposits, borrowing, repayments, and interactions with GlobalLedger
contract LendingPool is NilBase, NilTokenBase {
    address public globalLedger;
    address public interestManager;
    address public oracle;
    address public owner;
    TokenId public usdt;
    TokenId public eth;

    event DepositMade(address indexed user, TokenId token, uint256 amount);
    event LoanApproved(address indexed borrower, TokenId token, uint256 amount);
    event LoanRepaid(address indexed user, TokenId token, uint256 amount);

    constructor(
        address _globalLedger,
        address _interestManager,
        address _oracle,
        TokenId _usdt,
        TokenId _eth,
        address _owner
    ) {
        globalLedger = _globalLedger;
        interestManager = _interestManager;
        oracle = _oracle;
        usdt = _usdt;
        eth = _eth;
        owner = _owner;
    }

    /// @notice Deposit tokens into the lending pool
    function deposit() public payable {
        Nil.Token[] memory tokens = Nil.txnTokens();
        require(tokens.length > 0 && tokens[0].amount > 0, "Invalid deposit");

        bytes memory callData = abi.encodeWithSignature(
            "recordDeposit(address,address,uint256)",
            msg.sender,
            tokens[0].id,
            tokens[0].amount
        );
        Nil.asyncCall(globalLedger, address(this), 0, callData);

        emit DepositMade(msg.sender, tokens[0].id, tokens[0].amount);
    }

    /// @notice Borrow tokens by providing collateral
    function borrow(uint256 amount, TokenId borrowToken) public payable {
        require(borrowToken == usdt || borrowToken == eth, "Invalid token");
        require(Nil.tokenBalance(address(this), borrowToken) >= amount, "Insufficient funds");

        TokenId collateralToken = (borrowToken == usdt) ? eth : usdt;
        bytes memory callData = abi.encodeWithSignature("getPrice(address)", borrowToken);
        bytes memory context = abi.encodeWithSelector(
            this.processLoan.selector,
            msg.sender,
            amount,
            borrowToken,
            collateralToken
        );
        Nil.sendRequest(oracle, 0, 9_000_000, context, callData);
    }

    /// @notice Process the loan request after retrieving token price from Oracle
    function processLoan(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        require(success, "Oracle call failed");
        (address borrower, uint256 amount, TokenId borrowToken, TokenId collateralToken) =
            abi.decode(context, (address, uint256, TokenId, TokenId));

        uint256 borrowTokenPrice = abi.decode(returnData, (uint256));
        uint256 requiredCollateral = (amount * borrowTokenPrice * 120) / 100;

        bytes memory ledgerCallData = abi.encodeWithSignature(
            "getDeposit(address,address)",
            borrower,
            collateralToken
        );
        bytes memory ledgerContext = abi.encodeWithSelector(
            this.finalizeLoan.selector,
            borrower,
            amount,
            borrowToken,
            requiredCollateral
        );
        Nil.sendRequest(globalLedger, 0, 6_000_000, ledgerContext, ledgerCallData);
    }

    /// @notice Finalize the loan and transfer borrowed tokens
    function finalizeLoan(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        require(success, "Ledger call failed");
        (address borrower, uint256 amount, TokenId borrowToken, uint256 requiredCollateral) =
            abi.decode(context, (address, uint256, TokenId, uint256));

        uint256 userCollateral = abi.decode(returnData, (uint256));
        require(userCollateral >= requiredCollateral, "Insufficient collateral");

        bytes memory recordLoanCallData = abi.encodeWithSignature(
            "recordLoan(address,address,uint256)",
            borrower,
            borrowToken,
            amount
        );
        Nil.asyncCall(globalLedger, address(this), 0, recordLoanCallData);
        sendTokenInternal(borrower, borrowToken, amount);

        emit LoanApproved(borrower, borrowToken, amount);
    }

    /// @notice Repay a loan to reduce outstanding balance
    function repayLoan(uint256 amount, TokenId loanToken) public payable {
        require(amount > 0, "Invalid repayment amount");

        bytes memory callData = abi.encodeWithSignature(
            "getLoan(address,address)",
            msg.sender,
            loanToken
        );
        bytes memory context = abi.encodeWithSelector(
            this.finalizeRepayment.selector,
            msg.sender,
            amount,
            loanToken
        );
        Nil.sendRequest(globalLedger, 0, 5_000_000, context, callData);
    }

    /// @notice Finalize the loan repayment process
    function finalizeRepayment(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        require(success, "Ledger call failed");
        (address borrower, uint256 amount, TokenId loanToken) =
            abi.decode(context, (address, uint256, TokenId));

        uint256 outstandingLoan = abi.decode(returnData, (uint256));
        require(outstandingLoan > 0, "No active loan");
        require(outstandingLoan >= amount, "Repayment exceeds loan amount");

        sendTokenInternal(globalLedger, loanToken, amount);

        bytes memory repaymentCallData = abi.encodeWithSignature(
            "repayLoan(address,address,uint256)",
            borrower,
            loanToken,
            amount
        );
        Nil.asyncCall(globalLedger, address(this), 0, repaymentCallData);

        emit LoanRepaid(borrower, loanToken, amount);
    }
}