// SPDX-License-Identifier: GPL-3.0
pragma solidity ^0.8.9;

import "./nil/Nil.sol";
import "./nil/NilCurrencyBase.sol";

contract Currency {
    using Nil for address;

    string public name;
    bool public created;
    receive() external payable {}
    constructor(string memory _name) {
        name = _name;
        created = false;
    }

    function create(uint256 amount) external {
        require(!created, "Currency already created");
        address minterAddress = Nil.MINTER_ADDRESS;
        bytes memory callData = abi.encodeWithSignature(
            "create(uint256,address,string,address)",
            amount,
            address(this),
            name,
            address(this)
        );
        bool success = Nil.asyncCall(
            minterAddress,
            msg.sender,
            msg.sender,
            100000,
            0,
            false,
            0,
            callData
        );
        require(success, "Token creation failed");
        created = true;
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
