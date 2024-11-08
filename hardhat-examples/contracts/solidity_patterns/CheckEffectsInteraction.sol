// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract CheckEffectsInteractionParent is NilBase {

    mapping(address => uint) exampleBalances;
    uint public exampleBalance = exampleBalances[msg.sender];

    function topUpBalance(uint amount) public {
        exampleBalance += amount;
    }

    function checkEffectsInteraction(address dst, uint amount, bool value) public {
        // Check
        require(exampleBalance >= amount);

        // Effects
        exampleBalance -= amount;

        bytes memory context = abi.encodeWithSelector(this.callback.selector, amount);
        bytes memory callData = abi.encodeWithSignature("executed(bool)", value);

        // Interaction
        Nil.sendRequest(dst, 0, Nil.ASYNC_REQUEST_MIN_GAS, context, callData);
    }

    function callback(bool success, bytes memory returnData, bytes memory context) public payable onlyResponse {
        uint amount = abi.decode(context, (uint));
        if (!success) {
            exampleBalance += amount;
        }
    }
}

contract CheckEffectsInteractionChild is NilBase {
    function executed(bool value) public {
        require(true == value, "executed function failed");
    }
}
