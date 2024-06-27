// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./Minter.sol";

contract NilCurrencyBase is NilBase {
    function createToken(uint256 amount, string memory name, bool withdraw) public payable onlyExternal {
        uint256 gas = gasleft();
        Nil.asyncCall(
            Nil.MINTER_ADDRESS,
            address(0), // refundTo
            address(0), // bounceTo
            gas, // gas
            false, // deploy
            gas * 10, // value
            abi.encodeCall(Minter.create, (amount, address(0), name, withdraw ? address(this) : address(0)))
        );
    }

    function mintToken(uint256 amount, bool withdraw) public payable onlyExternal {
        uint256 id = uint256(uint160(address(this)));
        uint256 gas = 10000;
        Nil.asyncCall(
            Nil.MINTER_ADDRESS,
            address(0), // refundTo
            address(0), // bounceTo
            gas, // gas
            false, // deploy
            gas * 10, // value
            abi.encodeCall(Minter.mint, (id, amount, withdraw ? address(this) : address(0)))
        );
    }

    function withdrawToken(uint256 amount, address to) public payable onlyExternal {
        uint256 id = uint256(uint160(address(this)));
        uint256 gas = 50000;
        Nil.asyncCall(
            Nil.MINTER_ADDRESS,
            address(0), // refundTo
            address(0), // bounceTo
            gas, // gas
            false, // deploy
            2 * gas * 10, // value, 2x because transfer requires another async call
            abi.encodeCall(Minter.withdraw, (id, amount, to))
        );
    }
}
