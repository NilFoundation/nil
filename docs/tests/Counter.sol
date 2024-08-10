// SPDX-License-Identifier: MIT
//startContract
pragma solidity ^0.8.0;

contract Counter {
    uint256 private value;

    event ValueChanged(uint256 newValue);

    function increment() public {
        value += 1;
        emit ValueChanged(value);
    }

    function getValue() public view returns (uint256) {
        return value;
    }
}

//endContract
