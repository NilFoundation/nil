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
}
