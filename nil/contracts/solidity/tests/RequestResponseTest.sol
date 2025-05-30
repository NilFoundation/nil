// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "../lib/NilTokenBase.sol";
import "../lib/NilAwaitable.sol";
import "./Counter.sol";

contract RequestResponseTest is NilTokenBase, NilAwaitable {
    int32 public value;
    int32 public counterValue;
    uint public intValue;
    string public strValue;

    function verifyExternal(
        uint256,
        bytes calldata
    ) external pure returns (bool) {
        return true;
    }

    function reset() public {
        value = 0;
        counterValue = 0;
        intValue = 0;
        strValue = "";
    }

    function get() public view returns (int32) {
        return value;
    }

    function checkFail(bool fail) public pure {
        require(!fail, "Test for failed transaction");
    }

    /**
     * Test Counter's get method. Check context and return data.
     */
    function requestCounterGet(
        address counter,
        uint intContext,
        string memory strContext
    ) public async (500_000) {
        bytes memory context = abi.encode(intContext, strContext);
        bytes memory callData = abi.encodeWithSignature("get()");
        sendRequest(
            counter,
            0,
            Nil.ASYNC_REQUEST_MIN_GAS,
            context,
            callData,
            responseCounterGet
        );
    }

    function responseCounterGet(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public {
        require(success, "Request failed");
        (intValue, strValue) = abi.decode(context, (uint, string));
        counterValue = abi.decode(returnData, (int32));
    }

    /**
     * Nested sendRequest: request requestCounterGet which requests Counter.get
     */
    function nestedRequest(
        address callee,
        address counter
    ) public async (500_000) {
        bytes memory callData = abi.encodeWithSelector(this.requestCounterGet.selector, counter, 123, "test");
        sendRequest(
            callee,
            0,
            Nil.ASYNC_REQUEST_MIN_GAS,
            "",
            callData,
            responseNestedRequest
        );
    }

    function responseNestedRequest(
        bool success,
        bytes memory,
        bytes memory
    ) public pure {
        require(success, "Request failed");
    }

    /**
     * sendRequest from callback
     * Call Counter.Add(5), Counter.Add(4), Counter.Add(3), Counter.Add(2), Counter.Add(1)
     */
    function sendRequestFromCallback(
        address counter
    ) public async (500_000) {
        bytes memory context = abi.encode(int32(5), counter);
        bytes memory callData = abi.encodeWithSignature("add(int32)", 5);
        sendRequest(
            counter,
            0,
            Nil.ASYNC_REQUEST_MIN_GAS,
            context,
            callData,
            responseSendRequestFromCallback
        );
    }

    function responseSendRequestFromCallback(
        bool success,
        bytes memory,
        bytes memory context
    ) public async (500_000) {
        require(success, "Request failed");
        (int32 sendNext, address counter) = abi.decode(context, (int32, address));
        if (sendNext == 0) {
            return;
        }

        sendNext -= 1;

        context = abi.encode(sendNext, counter);
        bytes memory callData = abi.encodeWithSignature("add(int32)", sendNext);
        sendRequest(
            counter,
            0,
            Nil.ASYNC_REQUEST_MIN_GAS,
            context,
            callData,
            responseSendRequestFromCallback
        );
    }

    /**
     * Test Counter's add method. No context and empty return data.
     */
    function requestCounterAdd(address counter, int32 valueToAdd) public async (500_000) {
        bytes memory callData = abi.encodeWithSignature(
            "add(int32)",
            valueToAdd
        );
        sendRequest(
            counter,
            0,
            Nil.ASYNC_REQUEST_MIN_GAS,
            "",
            callData,
            responseCounterAdd
        );
    }

    function responseCounterAdd(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public view onlyResponse {
        require(success, "Request failed");
        require(context.length == 0, "Context should be empty");
        require(returnData.length == 0, "returnData should be empty");
    }

    /**
     * Test failure with value.
     */
    function requestCheckFail(address addr, bool fail) public async (500_000) {
        bytes memory context = abi.encode(uint(11111));
        bytes memory callData = abi.encodeWithSignature(
            "checkFail(bool)",
            fail
        );
        sendRequest(
            addr,
            1000000000,
            Nil.ASYNC_REQUEST_MIN_GAS,
            context,
            callData,
            responseCheckFail
        );
    }

    function responseCheckFail(
        bool success,
        bytes memory /*returnData*/,
        bytes memory context
    ) public payable {
        require(!success, "Request should be failed");
        uint ctxValue = abi.decode(context, (uint));
        require(ctxValue == uint(11111), "Context value should be the same");
    }

    /**
     * Test out of gas failure.
     */
    function requestOutOfGasFailure(address counter) public async (500_000) {
        bytes memory context = abi.encode(uint(1234567890));
        bytes memory callData = abi.encodeWithSignature("outOfGasFailure()");
        sendRequest(
            counter,
            0,
            Nil.ASYNC_REQUEST_MIN_GAS,
            context,
            callData,
            responseOutOfGasFailure
        );
    }

    function responseOutOfGasFailure(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public pure {
        require(!success, "Request should be failed");
        require(returnData.length == 0, "returnData should be empty");
        uint ctxValue = abi.decode(context, (uint));
        require(
            ctxValue == uint(1234567890),
            "Context value should be the same"
        );
    }

    function outOfGasFailure() public {
        while (true) {
            counterValue++;
        }
    }

    /**
     * Test token sending.
     */
    function requestSendToken(address addr, uint256 amount) public async (500_000) {
        bytes memory context = abi.encode(uint(11111));
        bytes memory callData = abi.encodeWithSignature("get()");
        Nil.Token[] memory tokens = new Nil.Token[](1);
        TokenId id = TokenId.wrap(address(this));
        tokens[0] = Nil.Token(id, amount);
        sendRequestWithTokens(
            addr,
            0,
            tokens,
            Nil.ASYNC_REQUEST_MIN_GAS,
            context,
            callData,
            responseSendToken
        );
    }

    function responseSendToken(
        bool success,
        bytes memory /*returnData*/,
        bytes memory context
    ) public payable {
        require(success, "Request should be successful");
        uint ctxValue = abi.decode(context, (uint));
        require(ctxValue == uint(11111), "Context value should be the same");
        require(Nil.txnTokens().length == 0, "Tokens should be empty");
    }

    /**
     * Fail during request sending. Context storage should not be changed.
     */
    function failDuringRequestSending(address counter) public async (500_000) {
        bytes memory context = abi.encode(intValue, strValue);
        bytes memory callData = abi.encodeWithSignature("get()");
        sendRequest(
            counter,
            0,
            Nil.ASYNC_REQUEST_MIN_GAS,
            context,
            callData,
            responseCounterGet
        );
        require(false, "Expect fail");
    }

    /**
     * Test two consecutive requests.
     */
    function makeTwoRequests(address addr1, address addr2) public async (1_000_000) {
        bytes memory callData = abi.encodeWithSignature("get()");
        sendRequest(addr1, 0, Nil.ASYNC_REQUEST_MIN_GAS, "", callData, makeTwoRequestsResponse);
        sendRequest(addr2, 0, Nil.ASYNC_REQUEST_MIN_GAS, "", callData, makeTwoRequestsResponse);
    }

    function makeTwoRequestsResponse(
        bool success,
        bytes memory returnData,
        bytes memory /*context*/
    ) public {
        require(success, "Request failed");
        value += abi.decode(returnData, (int32));
    }

    function makeInvalidSendRequest() public view {
        assembly {
            let memPtr := mload(0x40)
            let success := staticcall(3000, 0xd8, 0, 0, memPtr, 0x20)
        }
    }
}
