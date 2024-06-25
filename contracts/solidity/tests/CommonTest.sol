// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.9;

import "../Nil.sol";

contract CommonTest is NilBase {

    // Add output message, and then revert if `value` is zero. In that case output message should be removed.
    function testFailedAsyncCall(address dst, int32 value) onlyExternal public {
        uint256 gas = gasleft();
        Nil.asyncCall(dst, address(0), address(0), gas, false, gas * 10, abi.encodeWithSignature("add(int32)", 1));
        require(value != 0, "Value must be non-zero");
    }

    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }
}
