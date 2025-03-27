// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

/// @title LendingPool
/// @dev A contract for decentralized lending and borrowing, integrating GlobalLedger, InterestManager, and Oracle.
contract LendingPool is NilBase, NilTokenBase {
    address public globalLedger;
    address public interestManager;
    address public oracle;
    TokenId public usdt;
    TokenId public eth;

    /// @notice Constructor to initialize dependencies
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
    }

    /// @notice Deposit tokens into the lending pool.
    function deposit() public payable {
        Nil.Token[] memory tokens = Nil.txnTokens();
        bytes memory callData = abi.encodeWithSignature(
            "recordDeposit(address,address,uint256)",
            msg.sender,
            tokens[0].id,
            tokens[0].amount
        );
        Nil.asyncCall(globalLedger, address(this), 0, callData);
    }

    /// @notice Borrow tokens by providing collateral.
    function borrow(uint256 amount, TokenId borrowToken) public payable {
        require(borrowToken == usdt || borrowToken == eth, "Invalid token");
        require(
            Nil.tokenBalance(address(this), borrowToken) >= amount,
            "Insufficient funds"
        );

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

    /// @notice Process the loan request after retrieving token price from Oracle.
    function processLoan(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        require(success, "Oracle call failed");
        (address borrower, uint256 amount, TokenId borrowToken, TokenId collateralToken) = abi.decode(context, (address, uint256, TokenId, TokenId));
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

    /// @notice Finalize the loan and transfer borrowed tokens.
    function finalizeLoan(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        require(success, "Ledger call failed");
        (address borrower, uint256 amount, TokenId borrowToken, uint256 requiredCollateral) = abi.decode(context, (address, uint256, TokenId, uint256));
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
    }
}
