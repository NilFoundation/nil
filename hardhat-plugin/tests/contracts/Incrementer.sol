// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./Nil.sol";

contract Incrementer is NilBase {
    uint256 private value;

    event ValueChanged(uint256 newValue);
    receive() external payable {}

    function increment() public onlyInternal payable {
        value += 1;
        emit ValueChanged(value);
    }

    function getValue() public view returns (uint256) {
        return value;
    }
}

