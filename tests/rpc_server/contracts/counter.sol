// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

contract Counter {
    int32 value;

    function add(int32 val) public {
        value += val;
    }

    function get() public view returns(int32) {
        return value;
    }

    function verifyExternal(uint256, bytes memory) external view returns (bool) {
        return true;
    }
}
