// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

contract Counter {
    int32 value;

    event eventValue(int32 value);

    function add(int32 val) public {
        value += val;
    }

    function get() public returns(int32) {
        emit eventValue(value);
        return value;
    }

    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }
}
