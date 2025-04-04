// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@openzeppelin/contracts/access/Ownable.sol";

/// @title GlobalLedger
/// @dev Tracks deposits, loans, and repayments across LendingPool contracts
contract GlobalLedger is Ownable {
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
    mapping(address => bool) public authorizedLendingPools;

    event DepositRecorded(address indexed user, TokenId token, uint256 amount);
    event LoanRecorded(address indexed user, TokenId token, uint256 amount);
    event LoanRepaid(address indexed user, TokenId token, uint256 amount);
    event LendingPoolRegistered(address indexed lendingPool);

    /// @notice Modifier to restrict access to only authorized lending pools
    modifier onlyLendingPool() {
        require(
            authorizedLendingPools[msg.sender],
            "Not an authorized lending pool"
        );
        _;
    }

    /// @notice Register a new lending pool (only callable by factory or internal async call)
    function registerLendingPool(address lendingPool) external {
        require(
            msg.sender == owner() || msg.sender == address(this),
            "Unauthorized"
        );
        authorizedLendingPools[lendingPool] = true;
        emit LendingPoolRegistered(lendingPool);
    }

    /// @notice Record a deposit from an authorized LendingPool
    function recordDeposit(
        address user,
        TokenId token,
        uint256 amount
    ) external payable onlyLendingPool {
        require(amount > 0, "Invalid deposit amount");

        if (!deposits[user][token].exists) {
            deposits[user][token] = Deposit(amount, true);
        } else {
            deposits[user][token].amount += amount;
        }

        emit DepositRecorded(user, token, amount);
    }

    /// @notice Record a loan taken from an authorized LendingPool
    function recordLoan(
        address user,
        TokenId token,
        uint256 amount
    ) external payable onlyLendingPool {
        require(amount > 0, "Invalid loan amount");

        if (!loans[user][token].exists) {
            loans[user][token] = Loan(amount, true);
        } else {
            loans[user][token].amount += amount;
        }

        emit LoanRecorded(user, token, amount);
    }

    /// @notice Record loan repayment and reduce outstanding balance
    function repayLoan(
        address user,
        TokenId token,
        uint256 amount
    ) external payable onlyLendingPool {
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
