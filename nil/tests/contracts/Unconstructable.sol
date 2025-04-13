// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract Unconstructable {
    uint256 private value;

    constructor() {
        for (uint256 i = 0; i < 100; i++) {
            value += i;
        }
        require(false, "this contract cannot be constructed");
    }
}
