// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "../NilCurrencyBase.sol";

contract CounterPayable is NilCurrencyBase {
    int32 value;

    receive() external payable {}

    constructor() payable {
    }

    function add(int32 val) public payable {
        value += val;
    }

    function get() public view returns(int32) {
        return value;
    }

    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }
}
