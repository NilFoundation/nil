// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "../Nil.sol";

// Common test contract. Can be used in any test.
contract Test is NilBase {
    event stubCalled(uint32 v);

    uint32 private internalValue = 0;

    function getSum(uint a, uint b) public pure returns(uint) {
        return a + b;
    }

    function getString() public pure returns(string memory) {
        return "Very long string with many characters and words and spaces and numbers and symbols and everything else that can be in a string";
    }

    function getNumAndString() public pure returns(uint, string memory) {
        return (123456789012345678901234567890, "Simple string");
    }

    function getValue() public view returns(uint32) {
        return internalValue;
    }

    function setValue(uint32 newValue) public {
        internalValue = newValue;
    }

    function noReturn() public payable {}

    function nonPayable() public pure {}

    function mayRevert(bool isRevert) public payable {
        require(!isRevert, "Revert is true");
    }

    function proxyCall(address dst, uint gas, uint value, address refundTo, address bounceTo, bytes calldata callData) public payable {
        bool success = Nil.asyncCall(dst, refundTo, bounceTo, gas, Nil.FORWARD_REMAINING, false, value, callData);
        require(success, "Call failed");
    }

    struct AsyncCallArgs {
        address addr;
        uint feeCredit;
        uint8 forwardKind;
        address refundTo;
        bytes callData;
    }

    function testForwarding(AsyncCallArgs[] memory messages) public payable {
        for (uint i = 0; i < messages.length; i++) {
            AsyncCallArgs memory message = messages[i];
            bool success = Nil.asyncCall(message.addr, message.refundTo, address(this), message.feeCredit,
                message.forwardKind, false, 0, message.callData);
            require(success, "Call failed");
        }
    }

    function stub(uint n) public payable {
        emit stubCalled(uint32(n));
    }

    function getGasPrice() public returns(uint256) {
        return Nil.getGasPrice(address(this));
    }

    function getForwardKindRemaining() public pure returns(uint8) {
        return Nil.FORWARD_REMAINING;
    }

    function getForwardKindPercentage() public pure returns(uint8) {
        return Nil.FORWARD_PERCENTAGE;
    }

    function getForwardKindValue() public pure returns(uint8) {
        return Nil.FORWARD_VALUE;
    }

    function getForwardKindNone() public pure returns(uint8) {
        return Nil.FORWARD_NONE;
    }

    function bounce(string calldata err) external payable {}

    // Add output message, and then revert if `value` is zero. In that case output message should be removed.
    function testFailedAsyncCall(address dst, int32 value) onlyExternal public {
        Nil.asyncCall(dst, address(0), 0, abi.encodeWithSignature("add(int32)", 1));
        require(value != 0, "Value must be non-zero");
    }

    function getPoseidonHash(bytes memory data) public returns (uint256) {
        uint256 hash = Nil.getPoseidonHash(data);
        return hash;
    }

    function createAddress(uint shardId, bytes memory code, uint256 salt) public returns (address) {
        return Nil.createAddress(shardId, code, salt);
    }

    function createAddress2(uint shardId, address addr, uint256 salt, uint256 codeHash) public returns (address) {
        return Nil.createAddress2(shardId, addr, salt, codeHash);
    }

    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }
}
