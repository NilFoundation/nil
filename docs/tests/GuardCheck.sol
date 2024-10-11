// SPDX-License-Identifier: MIT
//startBadGuardCheck

pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract GuardCheck {
    uint256 successfulCallsCounter = 0;

    function badGuardCheckExample(address dst, uint256 amount) public payable {
        require(dst != address(0));
        require(msg.value != 0);
        require(msg.value > amount);
        uint balanceBeforeAsyncCall = address(this).balance;
        Nil.asyncCall(dst, address(this), amount, "");

        assert(address(this).balance == balanceBeforeAsyncCall - amount);
        successfulCallsCounter += 1;
    }
}

//endBadGuardCheck
//startGoodGuardCheck

contract GoodGuardCheck is NilBase {
    uint256 successfulCallsCounter = 0;
    address guardCheckerIntermediaryAddress;

    constructor(address _guardCheckerIntermediaryAddress) {
        guardCheckerIntermediaryAddress = _guardCheckerIntermediaryAddress;
    }

    function goodGuardCheckExample(address dst, uint256 amount) public payable {
        require(dst != address(0));
        require(msg.value != 0);
        require(msg.value > amount);
        uint balanceBeforeAsyncCall = address(this).balance;
        bytes memory context = abi.encodeWithSelector(this.callback.selector);
        bytes memory callData = abi.encodeWithSignature("receive()");
        Nil.sendRequest(
            dst,
            Nil.ASYNC_REQUEST_MIN_GAS,
            amount,
            context,
            callData
        );
        assert(address(this).balance == balanceBeforeAsyncCall - amount);
    }

    function callback(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public onlyResponse {
        require(success, "Transfer failed!");
        successfulCallsCounter += 1;
    }
}

//endGoodGuardCheck
