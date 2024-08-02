// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./Minter.sol";

contract NilCurrencyBase is NilBase {
    function createToken(uint256 amount, string memory name, bool withdraw) public payable onlyExternal {
        uint256 value = 0;
        if (withdraw) {
            value *= 2; // 2x because withdraw requires another async call
        }
        Nil.asyncCall(
            Nil.MINTER_ADDRESS,
            address(0), // refundTo
            address(0), // bounceTo
            0, // gas
            Nil.FORWARD_REMAINING, // forwardKind
            false, // deploy
            value, // value
            abi.encodeCall(Minter.create, (amount, address(0), name, withdraw ? address(this) : address(0)))
        );
    }

    function mintToken(uint256 amount, bool withdraw) public payable onlyExternal {
        uint256 id = uint256(uint160(address(this)));
        Nil.asyncCall(
            Nil.MINTER_ADDRESS,
            address(0), // refundTo
            address(0), // bounceTo
            0, // gas
            Nil.FORWARD_REMAINING, // forwardKind
            false, // deploy
            0, // value
            abi.encodeCall(Minter.mint, (id, amount, withdraw ? address(this) : address(0)))
        );
    }

    function withdrawToken(uint256 amount, address to) public payable onlyExternal {
        uint256 id = uint256(uint160(address(this)));
        Nil.asyncCall(
            Nil.MINTER_ADDRESS,
            address(0), // refundTo
            address(0), // bounceTo
            0, // gas
            Nil.FORWARD_REMAINING, // forwardKind
            false, // deploy
            0, // value
            abi.encodeCall(Minter.withdraw, (id, amount, to))
        );
    }
}
