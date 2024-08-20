// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "./nil/NilCurrencyBase.sol";

contract Currency is NilCurrencyBase {
    constructor(uint256 initialSupply) {
        // Mint the initial supply of tokens
        mintCurrencyInternal(initialSupply);
    }

    // Public function to call the parent internal function sendCurrencyInternal
    function transferCurrency(address to, uint256 currencyId, uint256 amount) public {
        sendCurrencyInternal(to, currencyId, amount);
    }

    function transfer(address destination, uint256 tokenId, uint256 amount) external {
        require(created, "Currency not created yet");

        // Create an array with one token to transfer
        Nil.Token[] memory tokens = new Nil.Token[](1);
        tokens[0] = Nil.Token({
            id: tokenId, // Use the tokenId passed as argument
            amount: amount
        });

        // Perform the async call without calldata
        bool success = Nil.asyncCall(
            destination,
            msg.sender,
            msg.sender,
            100000,
            0,
            false,
            0,
            tokens,
            ""
        );
        require(success, "Token transfer failed");
    }

    function transfer(address destination, uint256 tokenId, uint256 amount) external {
        require(created, "Currency not created yet");

        // Create an array with one token to transfer
        Nil.Token[] memory tokens = new Nil.Token[](1);
        tokens[0] = Nil.Token({
            id: tokenId, // Use the tokenId passed as argument
            amount: amount
        });

        // Perform the async call without calldata
        bool success = Nil.asyncCall(
            destination,
            msg.sender,
            msg.sender,
            100000,
            0,
            false,
            0,
            tokens,
            ""
        );
        require(success, "Token transfer failed");
    }
}
