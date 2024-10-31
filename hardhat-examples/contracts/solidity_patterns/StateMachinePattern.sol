// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.0;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract StateMachineParent is NilBase {

    enum State {
        RequestPending,
        InitialState,
        ResultReceived,
        ResultDisplayed
    }

    State public state = State.InitialState;
    bool public lock = false;

    function makeRequest(address dst, string memory funcName, bool executed) public  {
        if (lock) {
            Nil.asyncCall(dst, msg.sender, 0, abi.encodeWithSignature(funcName, executed));
            return;
        }
        lock = true;

        bytes memory context = abi.encodeWithSelector(this.callback.selector);
        bytes memory callData = abi.encodeWithSignature(funcName, executed);

        Nil.sendRequest(dst, 0, Nil.ASYNC_REQUEST_MIN_GAS, context, callData);
    }

    function callback(bool success, bytes memory returnData, bytes memory context) public payable onlyResponse {
        if (!success) {
            lock = false;
            return;
        }

        if (abi.decode(returnData, (bool))) {
            nextStage();
        }

        lock = false;
    }

    function nextStage() internal {
        state = State(uint(state) + 1);
    }

    function getState() public view returns (State) {
        if (lock) {
            return State.RequestPending;
        }
        return state;
    }
}

contract StateMachineChild is NilBase {
    function makeRequest(bool executed) public returns (bool) {
        return executed;
    }
}
