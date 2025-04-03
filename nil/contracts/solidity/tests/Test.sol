// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "../lib/Nil.sol";

// Common test contract. Can be used in any test.
contract Test is NilBase {
    event stubCalled(uint32 v);
    event testEvent(uint indexed a, uint indexed b);

    uint32 private internalValue = 0;
    uint256 private timestamp = 0;

    constructor() payable {}

    function garbageInRequire(bool f, string memory m) public pure {
        require(f, m);
    }

    function emitEvent(uint a, uint b) public payable {
        emit testEvent(a, b);
    }

    function getSum(uint a, uint b) public pure returns (uint) {
        return a + b;
    }

    function getString() public pure returns (string memory) {
        return
            "Very long string with many characters and words and spaces and numbers and symbols and everything else that can be in a string";
    }

    function getNumAndString() public pure returns (uint, string memory) {
        return (123456789012345678901234567890, "Simple string");
    }

    function getValue() public view returns (uint32) {
        return internalValue;
    }

    function setValue(uint32 newValue) public {
        internalValue = newValue;
        emit stubCalled(newValue);

        int[] memory arr = new int[](1);
        arr[0] = int(uint256(newValue));
        Nil.log("Value set to", arr);
    }

    function burnGas() public payable {
        uint256[] memory data = new uint256[](2 ** 24);
        require(false, "Intentional failure");
        internalValue = uint32(data.length);
    }

    function noReturn() public payable {}

    function nonPayable() public pure {}

    function mayRevert(bool isRevert) public payable {
        require(!isRevert, "Revert is true");
    }

    function proxyCall(
        address dst,
        uint gas,
        uint value,
        address refundTo,
        address bounceTo,
        bytes calldata callData
    ) public payable {
        Nil.asyncCall(
            dst,
            refundTo,
            bounceTo,
            gas,
            Nil.FORWARD_REMAINING,
            value,
            callData
        );
    }

    struct AsyncCallArgs {
        address addr;
        uint feeCredit;
        uint8 forwardKind;
        address refundTo;
        bytes callData;
    }

    function testForwarding(
        AsyncCallArgs[] memory transactions
    ) public payable {
        for (uint i = 0; i < transactions.length; i++) {
            AsyncCallArgs memory transaction = transactions[i];
            Nil.asyncCall(
                transaction.addr,
                transaction.refundTo,
                address(this),
                transaction.feeCredit,
                transaction.forwardKind,
                0,
                transaction.callData
            );
        }
    }

    function testForwardingInSendRawTransaction(
        bytes memory transaction
    ) public payable {
        Nil.sendTransaction(transaction);
    }

    function stub(uint n) public payable {
        emit stubCalled(uint32(n));
    }

    function getGasPrice() public returns (uint256) {
        return Nil.getGasPrice(address(this));
    }

    function getForwardKindRemaining() public pure returns (uint8) {
        return Nil.FORWARD_REMAINING;
    }

    function getForwardKindPercentage() public pure returns (uint8) {
        return Nil.FORWARD_PERCENTAGE;
    }

    function getForwardKindValue() public pure returns (uint8) {
        return Nil.FORWARD_VALUE;
    }

    function getForwardKindNone() public pure returns (uint8) {
        return Nil.FORWARD_NONE;
    }

    function bounce(string calldata err) external payable {}

    function saveTime() public {
        timestamp = block.timestamp;
    }

    // Add output transaction, and then revert if `value` is zero. In that case output transaction should be removed.
    function testFailedAsyncCall(address dst, int32 value) public onlyExternal {
        Nil.asyncCall(
            dst,
            address(0),
            0,
            abi.encodeWithSignature("add(int32)", 1)
        );
        require(value != 0, "Value must be non-zero");
    }

    function getPoseidonHash(bytes memory data) public returns (uint256) {
        uint256 hash = Nil.getPoseidonHash(data);
        return hash;
    }

    function createAddress(
        uint shardId,
        bytes memory code,
        uint256 salt
    ) public returns (address) {
        return Nil.createAddress(shardId, code, salt);
    }

    function createAddress2(
        uint shardId,
        address addr,
        uint256 salt,
        uint256 codeHash
    ) public returns (address) {
        return Nil.createAddress2(shardId, addr, salt, codeHash);
    }

    // Currently, functions below are used for manual testing of the Cometa service.
    function callUnknown() public {
        bytes memory returnData;
        bool success;
        bytes memory callData = abi.encodeWithSignature("nonexistent()");
        (returnData, success) = Nil.awaitCall(
            address(this),
            Nil.ASYNC_REQUEST_MIN_GAS,
            callData
        );
        require(success, "awaitCall failed");
    }

    function twoCalls(address addr1, address addr2) public {
        bytes memory context = abi.encodeWithSelector(
            this.responseCounterGet.selector
        );
        bytes memory callData = abi.encodeWithSignature("get()");
        Nil.sendRequest(addr1, 0, Nil.ASYNC_REQUEST_MIN_GAS, context, callData);
        Nil.sendRequest(addr2, 0, Nil.ASYNC_REQUEST_MIN_GAS, context, callData);
    }

    function responseCounterGet(
        bool success,
        bytes memory returnData,
        bytes memory /*context*/
    ) public {
        require(success, "Request failed");
        internalValue = uint32(abi.decode(returnData, (int32)));
    }

    function fibonacciWithFail(int32 n, int32 failN) public returns (int32) {
        if (n == failN) {
            revert("Fail because of `n == failN`");
        }
        if (n <= 1) {
            return n;
        }
        bytes memory returnData;
        bytes memory callData;
        bool success;
        callData = abi.encodeWithSignature(
            "fibonacciWithFail(int32,int32)",
            n - 1,
            failN
        );
        (returnData, success) = Nil.awaitCall(
            address(this),
            Nil.ASYNC_REQUEST_MIN_GAS,
            callData
        );
        require(success, "awaitCall 1 failed");
        int32 a = abi.decode(returnData, (int32));

        callData = abi.encodeWithSignature(
            "fibonacciWithFail(int32,int32)",
            n - 2,
            failN
        );
        (returnData, success) = Nil.awaitCall(
            address(this),
            Nil.ASYNC_REQUEST_MIN_GAS,
            callData
        );
        require(success, "awaitCall 2 failed");
        int32 b = abi.decode(returnData, (int32));

        return a + b;
    }

    /**
     * Test that performs a request that always throws empty error message.
     */
    function returnEmptyError() public pure {
        require(false, "");
    }

    function makeFail(int32 n) public pure returns (int32) {
        if (n == 1) {
            int32 v = abi.decode(bytes(""), (int32));
            require(v != 0);
        }
        return 0;
    }

    function emitLog(string memory transaction, bool fail) public {
        Nil.log(transaction);
        int[] memory arr = new int[](2);
        arr[0] = 8888;
        arr[1] = fail ? int(1) : int(0);
        Nil.log(transaction, arr);
        emit testEvent(1, 2);
        require(!fail, "Fail is true");
    }

    function verifyExternal(
        uint256,
        bytes calldata
    ) external pure returns (bool) {
        return true;
    }
}

contract HeavyConstructor {
    uint256 public value;

    // Consumes gas by using hot SSTORE(~529 gas per iteration)
    constructor(uint256 n) payable {
        for (uint256 i = 1; i < n; i++) {
            value *= 2;
        }
    }
}

contract Empty {}
