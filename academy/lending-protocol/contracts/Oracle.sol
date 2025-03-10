// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

/// @title Oracle
/// @dev The Oracle contract provides token price data to the lending protocol.
/// It is used to fetch the price of tokens (e.g., USDT, ETH) used for collateral calculations in the lending process.
contract Oracle is NilBase {
    /// @dev Mapping to store the price of each token (TokenId => price).
    mapping(TokenId => uint256) public rates;

    /// @notice Set the price of a token.
    /// @dev This function allows the price of tokens to be updated in the Oracle contract.
    ///      Only authorized entities (e.g., the contract owner or admin) should be able to set the price.
    /// @param token The token whose price is being set.
    /// @param price The new price of the token.
    function setPrice(TokenId token, uint256 price) public {
        /// @notice Store the price of the token in the rates mapping.
        /// @dev This updates the price of the specified token in the Oracle contract.
        rates[token] = price;
    }

    /// @notice Retrieve the price of a token.
    /// @dev This function allows other contracts (like LendingPool) to access the current price of a token.
    /// @param token The token whose price is being fetched.
    /// @return uint256 The price of the specified token.
    function getPrice(TokenId token) public view returns (uint256) {
        /// @notice Return the price of the specified token.
        /// @dev This function provides the price stored in the `rates` mapping for the given token.
        return rates[token];
    }
}
