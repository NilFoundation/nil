// SPDX-License-Identifier: MIT
//startBadStateMachine

pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract StateMachine {
    enum State {
        InitialState,
        RequestPending,
        ResultReceived,
        ResultDisplayed
    }

    State public state = State.InitialState;

    function makeRequest(address dst, uint id) public {
        Nil.asyncCall(
            dst,
            address(this),
            0,
            abi.encodeWithSignature("getData(uint)", id)
        );

        nextStage();
    }

    function nextStage() internal {
        state = State(uint(state) + 1);
    }
}

//endBadStateMachine

//startGoodStateMachine

contract GoodStateMachine is NilBase {
    enum State {
        InitialState,
        RequestPending,
        ResultReceived,
        ResultDisplayed
    }

    State public state = State.InitialState;

    mapping(State => address) delegates;

    constructor(address[4] memory _delegateAddresses) {
        for (uint i; i < 4; i++) {
            delegates[State(i)] = _delegateAddresses[i];
        }
    }

    function makeRequestToStateContract() public {
        address dst = delegates[State(uint(state) + 1)];
        bytes memory context = abi.encodeWithSelector(this.callback.selector);
        bytes memory callData = abi.encodeWithSignature("makeRequest()");
        Nil.sendRequest(
            dst,
            500_000,
            Nil.ASYNC_REQUEST_MIN_GAS,
            context,
            callData
        );
    }

    function callback(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public onlyResponse {
        require(success, "Request failed!");
        nextStage();
    }

    function nextStage() internal {
        state = State(uint(state) + 1);
    }
}

contract StateMachineDelegate is NilBase {
  function makeRequest() public onlyInternal {
    ...
  }
}

//endGoodStateMachine
