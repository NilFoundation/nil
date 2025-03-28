// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

// @title GlobalLedger
// @dev Tracks deposits, loans, and repayments across LendingPool contracts
contract GlobalLedger {
    struct Deposit {
        uint256 amount;
        bool exists;
    }

    struct Loan {
        uint256 amount;
        bool exists;
    }

    mapping(address => mapping(TokenId => Deposit)) public deposits;
    mapping(address => mapping(TokenId => Loan)) public loans;

    event DepositRecorded(address indexed user, TokenId token, uint256 amount);
    event LoanRecorded(address indexed user, TokenId token, uint256 amount);
    event LoanRepaid(address indexed user, TokenId token, uint256 amount);

    /// @notice Record a deposit from LendingPool
    function recordDeposit(address user, TokenId token, uint256 amount) external payable {
        require(amount > 0, "Invalid deposit amount");

        if (!deposits[user][token].exists) {
            deposits[user][token] = Deposit(amount, true);
        } else {
            deposits[user][token].amount += amount;
        }

        emit DepositRecorded(user, token, amount);
    }

    /// @notice Get user's deposit for a specific token
    function getDeposit(address user, TokenId token) external view returns (uint256) {
        return deposits[user][token].amount;
    }

    /// @notice Record a loan taken from a LendingPool
    function recordLoan(address user, TokenId token, uint256 amount) external payable {
        require(amount > 0, "Invalid loan amount");

        if (!loans[user][token].exists) {
            loans[user][token] = Loan(amount, true);
        } else {
            loans[user][token].amount += amount;
        }

        emit LoanRecorded(user, token, amount);
    }

    /// @notice Get user's outstanding loan for a specific token
    function getLoan(address user, TokenId token) external view returns (uint256) {
        return loans[user][token].amount;
    }

    /// @notice Record loan repayment and reduce outstanding balance
    function repayLoan(address user, TokenId token, uint256 amount) external payable {
        require(amount > 0, "Invalid repayment amount");
        require(loans[user][token].exists, "No active loan");

        if (loans[user][token].amount <= amount) {
            delete loans[user][token];
        } else {
            loans[user][token].amount -= amount;
        }

        emit LoanRepaid(user, token, amount);
    }
}