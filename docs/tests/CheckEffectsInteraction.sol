// SPDX-License-Identifier: MIT

//startEscrow
pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract CheckEffectsInteraction is NilBase {
    //startBadCheckEffectsInteraction
    mapping(address => uint) balances;

    function badCheckEffectsInteraction(address dst, uint amount) public {
        require(balances[msg.sender] >= amount);

        balances[msg.sender] -= amount;

        Nil.asyncCall(dst, address(this), amount, "");
    }

    //endBadCheckEffectsInteraction

    //startGoodCheckEffectsInteraction
    mapping(address => uint) exampleBalances;

    function goodCheckEffectInteration(address dst, uint amount) public {
        require(exampleBalances[msg.sender] >= amount);
        exampleBalances[msg.sender] -= amount;

        bytes memory context = abi.encodeWithSelector(
            this.callback.selector,
            amount
        );
        Nil.sendRequest(dst, amount, Nil.ASYNC_REQUEST_MIN_GAS, context, "");
    }

    function callback(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable onlyResponse {
        uint amount = abi.decode(context, (uint));
        if (!success) {
            exampleBalances[msg.sender] += amount;
        }
    }

    //endGoodCheckEffectsInteraction
}
