// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "../lib/NilCurrencyBase.sol";
import "./Counter.sol";

contract RequestResponseTest is NilCurrencyBase {
    int32 public value;
    int32 public counterValue;
    uint public intValue;
    string public strValue;

    event awaitCallResult(bool result);

    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }

    /**
     * Test sum of counters via awaitCall.
     */
    function sumCounters(address[] memory counters) public {
        for (uint i = 0; i < counters.length; i++) {
            bytes memory callData = abi.encodeWithSignature("get()");

            (bytes memory returnData, bool success) = Nil.awaitCall(counters[i], 60_000, callData);

            require(success, "awaitCall failed");
            int32 counterVal = abi.decode(returnData, (int32));
            value += counterVal;
        }
    }

    function get() public view returns(int32) {
        return value;
    }

    /**
     * Test awaitCall for failed method.
     */
    function callFailed(address addr, bool fail) public {
        bytes memory callData = abi.encodeWithSignature("checkFail(bool)", fail);
        (, bool success) = Nil.awaitCall(addr, Nil.ASYNC_REQUEST_MIN_GAS, callData);
        emit awaitCallResult(success);
    }

    function checkFail(bool fail) public pure {
        require(!fail, "Test for failed transaction");
    }

    /**
     * Test factorial implementation via awaitCall.
     */
    function factorial(int32 n) public {
        value = factorialRec(n);
    }

    function factorialRec(int32 n) public returns(int32) {
        if (n == 0) {
            return 1;
        }
        bytes memory callData = abi.encodeWithSignature("factorialRec(int32)", n - 1);
        (bytes memory returnData, bool success) = Nil.awaitCall(address(this), Nil.ASYNC_REQUEST_MIN_GAS, callData);
        require(success, "awaitCall failed");
        int32 prev = abi.decode(returnData, (int32));
        return n * prev;
    }

    /**
     * Test fibonacci implementation via awaitCall.
     * Here we have two awaitCall in a row, so it should be properly handled by the system.
     */
    function fibonacci(int32 n) public {
        value = fibonacciRec(n);
    }

    function fibonacciRec(int32 n) public returns(int32) {
        if (n <= 1) {
            return n;
        }
        bytes memory returnData;
        bytes memory callData;
        bool success;
        callData = abi.encodeWithSignature("fibonacciRec(int32)", n - 1);
        (returnData, success) = Nil.awaitCall(address(this), Nil.ASYNC_REQUEST_MIN_GAS, callData);
        require(success, "awaitCall 1 failed");
        int32 a = abi.decode(returnData, (int32));

        callData = abi.encodeWithSignature("fibonacciRec(int32)", n - 2);
        (returnData, success) = Nil.awaitCall(address(this), Nil.ASYNC_REQUEST_MIN_GAS, callData);
        require(success, "awaitCall 2 failed");
        int32 b = abi.decode(returnData, (int32));

        return a + b;
    }

    /**
     * Test nested sum of counters via awaitCall.
     */
    function sumCountersNested(address[] memory tests, address[] memory counters) public {
        for (uint i = 0; i < tests.length; i++) {
            bytes memory callData = abi.encodeWithSignature("awaitGet(address)", counters[i]);
            (bytes memory returnData, bool success) = Nil.awaitCall(tests[i], Nil.ASYNC_REQUEST_MIN_GAS, callData);

            require(success, "awaitCall failed");
            int32 counterVal = abi.decode(returnData, (int32));
            value += counterVal;
        }
    }

    function awaitGet(address counter) public returns(int32) {
        bytes memory callData = abi.encodeWithSignature("get()");
        (bytes memory returnData, bool success) = Nil.awaitCall(counter, Nil.ASYNC_REQUEST_MIN_GAS, callData);
        require(success, "awaitCall failed");
        return abi.decode(returnData, (int32));
    }

    /**
     * Test Counter's get method. Check context and return data.
     */
    function requestCounterGet(address counter, uint intContext, string memory strContext) public {
        bytes memory context = abi.encodeWithSelector(this.responseCounterGet.selector, intContext, strContext);
        bytes memory callData = abi.encodeWithSignature("get()");
        Nil.sendRequest(counter, 0, Nil.ASYNC_REQUEST_MIN_GAS, context, callData);
    }

    function responseCounterGet(bool success, bytes memory returnData, bytes memory context) public {
        require(success, "Request failed");
        (intValue, strValue) = abi.decode(context, (uint, string));
        counterValue = abi.decode(returnData, (int32));
    }

    /**
     * Test Counter's add method. No context and empty return data.
     */
    function requestCounterAdd(address counter, int32 valueToAdd) public {
        bytes memory context = abi.encodeWithSelector(this.responseCounterAdd.selector);
        bytes memory callData = abi.encodeWithSignature("add(int32)", valueToAdd);
        Nil.sendRequest(counter, 0, Nil.ASYNC_REQUEST_MIN_GAS, context, callData);
    }

    function responseCounterAdd(bool success, bytes memory returnData, bytes memory context) public view onlyResponse {
        require(success, "Request failed");
        require(context.length == 0, "Context should be empty");
        require(returnData.length == 0, "returnData should be empty");
    }

    /**
     * Test failure with value.
     */
    function requestCheckFail(address addr, bool fail) public {
        bytes memory context = abi.encodeWithSelector(this.responseCheckFail.selector, uint(11111));
        bytes memory callData = abi.encodeWithSignature("checkFail(bool)", fail);
        Nil.sendRequest(addr, 1000000000, Nil.ASYNC_REQUEST_MIN_GAS, context, callData);
    }

    function responseCheckFail(bool success, bytes memory /*returnData*/, bytes memory context) public payable {
        require(!success, "Request should be failed");
        (uint ctxValue) = abi.decode(context, (uint));
        require(ctxValue == uint(11111), "Context value should be the same");
    }

    /**
     * Test out of gas failure.
     */
    function requestOutOfGasFailure(address counter) public {
        bytes memory context = abi.encodeWithSelector(this.responseOutOfGasFailure.selector, uint(1234567890));
        bytes memory callData = abi.encodeWithSignature("outOfGasFailure()");
        Nil.sendRequest(counter, 0, Nil.ASYNC_REQUEST_MIN_GAS, context, callData);
    }

    function responseOutOfGasFailure(bool success, bytes memory returnData, bytes memory context) public pure {
        require(!success, "Request should be failed");
        require(returnData.length == 0, "returnData should be empty");
        (uint ctxValue) = abi.decode(context, (uint));
        require(ctxValue == uint(1234567890), "Context value should be the same");
    }

    function outOfGasFailure() public {
        while (true) {
            counterValue++;
        }
    }

    /**
     * Test currency sending.
     */
    function requestSendCurrency(address addr, uint256 amount) public {
        bytes memory context = abi.encodeWithSelector(this.responseSendCurrency.selector, uint(11111));
        bytes memory callData = abi.encodeWithSignature("get()");
        Nil.Token[] memory tokens = new Nil.Token[](1);
        CurrencyId id = CurrencyId.wrap(address(this));
        tokens[0] = Nil.Token(id, amount);
        Nil.sendRequestWithTokens(addr, 0, tokens, Nil.ASYNC_REQUEST_MIN_GAS, context, callData);
    }

    function responseSendCurrency(bool success, bytes memory /*returnData*/, bytes memory context) public payable {
        require(success, "Request should be successful");
        (uint ctxValue) = abi.decode(context, (uint));
        require(ctxValue == uint(11111), "Context value should be the same");
        require(Nil.msgTokens().length == 0, "Tokens should be empty");
    }

    /**
     * Fail during request sending. Context storage should not be changed.
     */
    function failDuringRequestSending(address counter) public {
        bytes memory context = abi.encodeWithSelector(this.responseCounterGet.selector, intValue, strValue);
        bytes memory callData = abi.encodeWithSignature("get()");
        Nil.sendRequest(counter, 0, Nil.ASYNC_REQUEST_MIN_GAS, context, callData);
        require(false, "Expect fail");
    }

    /**
     * Should fail because `awaitCall` can be used only in top-level functions.
     */
    function testNoneZeroCallDepth(address addr) public {
        RequestResponseTest(addr).awaitGet(address(this));
    }

    /**
     * Test two consecutive requests.
     */
    function makeTwoRequests(address addr1, address addr2) public {
        bytes memory context = abi.encodeWithSelector(this.makeTwoRequestsResponse.selector);
        bytes memory callData = abi.encodeWithSignature("get()");
        Nil.sendRequest(addr1, 0, Nil.ASYNC_REQUEST_MIN_GAS, context, callData);
        Nil.sendRequest(addr2, 0, Nil.ASYNC_REQUEST_MIN_GAS, context, callData);
    }

    function makeTwoRequestsResponse(bool success, bytes memory returnData, bytes memory /*context*/) public {
        require(success, "Request failed");
        value += abi.decode(returnData, (int32));
    }

    function makeInvalidContext(address addr1) public {
        bytes memory context = new bytes(1);
        bytes memory callData = new bytes(1);
        Nil.sendRequest(addr1, 0, Nil.ASYNC_REQUEST_MIN_GAS, context, callData);
    }
}
