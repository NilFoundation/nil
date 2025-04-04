// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

// Contract to handle collateral manager functionality and embedded GlobalLedger
contract CollateralManager {
    // Define the role-based access modifier for Factory control
    address public lendingPoolFactory;

    /// @dev Mapping of user addresses to their token deposits (token -> amount).
    mapping(address => mapping(TokenId => uint256)) public deposits;

    /// @dev Mapping of user addresses to their loans (loan amount and loan token).
    mapping(address => Loan) public loans;

    /// @dev Struct to store loan details: amount and the token type.
    struct Loan {
        uint256 amount;
        TokenId token;
    }

    /// @notice Modifier to restrict access to only the lending pool factory
    modifier onlyLendingPoolFactory() {
        require(
            msg.sender == lendingPoolFactory,
            "Only LendingPoolFactory can call this"
        );
        _;
    }

    // Constructor to set the factory address
    constructor(address _lendingPoolFactory) {
        lendingPoolFactory = _lendingPoolFactory;
    }

    /// @notice Records a user's deposit into the ledger.
    /// @dev Increases the deposit balance for the user for the specified token.
    /// @param user The address of the user making the deposit.
    /// @param token The token type being deposited (e.g., USDT, ETH).
    /// @param amount The amount of the token being deposited.
    function recordDeposit(
        address user,
        TokenId token,
        uint256 amount
    ) public onlyLendingPoolFactory {
        deposits[user][token] += amount;
    }

    /// @notice Fetches a user's deposit balance for a specific token.
    /// @dev Returns the amount of the token deposited by the user.
    /// @param user The address of the user whose deposit balance is being fetched.
    /// @param token The token type for which the balance is being fetched.
    /// @return uint256 The deposit amount for the given user and token.
    function getDeposit(
        address user,
        TokenId token
    ) public view returns (uint256) {
        return deposits[user][token]; // Return the deposit amount for the given user and token
    }

    /// @notice Records a user's loan in the ledger.
    /// @dev Stores the amount of the loan and the token type used for the loan.
    /// @param user The address of the user taking the loan.
    /// @param token The token type used for the loan (e.g., USDT, ETH).
    /// @param amount The amount of the loan being taken.
    function recordLoan(
        address user,
        TokenId token,
        uint256 amount
    ) public onlyLendingPoolFactory {
        loans[user] = Loan(amount, token);
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

    /// @notice Allows the lending pool factory to set the factory address if required
    /// @param newFactory Address of the new lending pool factory
    function setLendingPoolFactory(
        address newFactory
    ) external onlyLendingPoolFactory {
        lendingPoolFactory = newFactory;
    }
}
