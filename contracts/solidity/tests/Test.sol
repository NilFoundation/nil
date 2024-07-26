// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "../Nil.sol";

// Common test contract. Can be used in any test.
contract Test is NilBase {

    function getSum(uint a, uint b) public pure returns(uint) {
        return a + b;
    }

    function getString() public pure returns(string memory) {
        return "Very long string with many characters and words and spaces and numbers and symbols and everything else that can be in a string";
    }

    function getNumAndString() public pure returns(uint, string memory) {
        return (123456789012345678901234567890, "Simple string");
    }

    function noReturn() public payable {}

    function nonPayable() public pure {}

    function mayRevert(bool isRevert) public payable {
        require(!isRevert, "Revert is true");
    }

    function proxyCall(address dst, uint gas, uint value, address refundTo, address bounceTo, bytes calldata callData) public payable {
        bool success = Nil.asyncCall(dst, refundTo, bounceTo, gas, false, value, callData);
        require(success, "Call failed");
    }

    function getGasPrice() public returns(uint256) {
        return Nil.getGasPrice(address(this));
    }

    function bounce(string calldata err) external payable {}

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
