// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "../Nil.sol";
import "./Counter.sol";

contract AwaitTest {
    int32 public value;

    event awaitCallResult(bool result);

    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }

    function sumCounters(address[] memory counters) public {
        for (uint i = 0; i < counters.length; i++) {
            bytes memory callData = abi.encodeWithSignature("get()");

            (bytes memory returnData, bool success) = Nil.awaitCall(counters[i], callData);

            require(success, "awaitCall failed");
            int32 counterVal = abi.decode(returnData, (int32));
            value += counterVal;
        }
    }

    function get() public view returns(int32) {
        return value;
    }

    function callFailed(address addr, bool fail) public {
        bytes memory callData = abi.encodeWithSignature("checkFail(bool)", fail);
        (, bool success) = Nil.awaitCall(addr, callData);
        emit awaitCallResult(success);
    }

    function checkFail(bool fail) public pure {
        require(!fail, "Test for failed transaction");
    }

    function factorial(int32 n) public {
        value = factorialRec(n);
    }

    function factorialRec(int32 n) public returns(int32) {
        if (n == 0) {
            return 1;
        }
        bytes memory callData = abi.encodeWithSignature("factorialRec(int32)", n - 1);
        (bytes memory returnData, bool success) = Nil.awaitCall(address(this), callData);
        require(success, "awaitCall failed");
        int32 prev = abi.decode(returnData, (int32));
        return n * prev;
    }

    function sumCountersNested(address[] memory tests, address[] memory counters) public {
        for (uint i = 0; i < tests.length; i++) {
            bytes memory callData = abi.encodeWithSignature("awaitGet(address)", counters[i]);
            (bytes memory returnData, bool success) = Nil.awaitCall(tests[i], callData);

            require(success, "awaitCall failed");
            int32 counterVal = abi.decode(returnData, (int32));
            value += counterVal;
        }
    }

    function awaitGet(address counter) public returns(int32) {
        bytes memory callData = abi.encodeWithSignature("get()");
        (bytes memory returnData, bool success) = Nil.awaitCall(counter, callData);
        require(success, "awaitCall failed");
        return abi.decode(returnData, (int32));
    }
}
