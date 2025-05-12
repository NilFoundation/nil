// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./Nil.sol";

contract NilAwaitable is NilBase {
    Control internal ctrl;

    address private zeroAddress = address(0);

    struct Awaiter {
        function(bool, bytes memory, bytes memory) internal callback;
        uint answer_id;
        bool active;
        bytes context;
    }

    struct Control {
        mapping(uint256=>Awaiter) awaiters;
        uint256 await_id;
    }

    /**
     * @dev Sends a request to a contract.
     * @param dst Destination address of the request.
     * @param value Value to be sent with the request.
     * @param responseProcessingGas Amount of gas is being bought and reserved to process the response.
     *        Should be >= `ASYNC_REQUEST_MIN_GAS` to make a call, otherwise `sendRequest` will fail.
     * @param context Context data that is preserved in order to be available in the response method.
     * @param callData Calldata for the request.
     * @param cb Callback to be called when the response is received.
     */
    function sendRequest(
        address dst,
        uint256 value,
        uint responseProcessingGas,
        bytes memory context,
        bytes memory callData,
        function(bool, bytes memory, bytes memory) internal cb
    ) internal {
        Nil.Token[] memory tokens;
        sendRequestWithTokens(dst, value, tokens, responseProcessingGas, context, callData, cb);
    }

    /**
     * @dev Sends a request to a contract with tokens.
     * @param dst Destination address of the request.
     * @param value Value to be sent with the request.
     * @param tokens Array of tokens to be sent with the request.
     * @param responseProcessingGas Amount of gas is being bought and reserved to process the response.
     *        should be >= `ASYNC_REQUEST_MIN_GAS` to make a call, otherwise `sendRequest` will fail.
     * @param context Context data that is preserved in order to be available in the response method.
     * @param callData Calldata for the request.
     * @param cb Callback to be called when the response is received.
     */
    function sendRequestWithTokens(
        address dst,
        uint256 value,
        Nil.Token[] memory tokens,
        uint responseProcessingGas,
        bytes memory context,
        bytes memory callData,
        function(bool, bytes memory, bytes memory) internal cb
    ) internal {
        ctrl.await_id += 1;
        ctrl.awaiters[ctrl.await_id] = Awaiter({callback: cb, answer_id: ctrl.await_id, active: true, context: context});
        Nil.asyncCallWithTokens(
            dst,
            zeroAddress,
            zeroAddress,
            0,
            Nil.FORWARD_REMAINING,
            value,
            tokens,
            callData,
            ctrl.await_id,
            responseProcessingGas
        );
    }

    function onFallback(uint256 answer_id, bool success, bytes memory response) external payable {
        Awaiter storage awaiter = ctrl.awaiters[answer_id];
        require(awaiter.active);
        function(bool, bytes memory, bytes memory) internal cb = awaiter.callback;
        bytes memory context = awaiter.context;
        delete ctrl.awaiters[answer_id];
        cb(success, response, context);
    }
}
