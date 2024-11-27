// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract Incrementer {
    uint256 private value;

    constructor(uint256 initialValue) {
        value = initialValue;
    }

    function increment() public {
        value += 1;
    }

    receive() external payable {}

    function get() public view returns(uint256) {
        return value;
    }
}
