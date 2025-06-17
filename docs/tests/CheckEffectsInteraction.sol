// SPDX-License-Identifier: MIT

//startEscrow
pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilAwaitable.sol";

contract CheckEffectsInteraction is NilBase, NilAwaitable {
    //startBadCheckEffectsInteraction
    mapping(address => uint) balances;

    function badCheckEffectsInteraction(address dst, uint amount) public async(2_000_000) {
        require(balances[Nil.msgSender()] >= amount);

        balances[Nil.msgSender()] -= amount;

        Nil.asyncCall(dst, address(this), amount, "");
    }

    //endBadCheckEffectsInteraction

    //startGoodCheckEffectsInteraction
    mapping(address => uint) exampleBalances;

    function goodCheckEffectInteration(address dst, uint amount) public async(2_000_000) {
        require(exampleBalances[Nil.msgSender()] >= amount);
        exampleBalances[Nil.msgSender()] -= amount;

        bytes memory context = abi.encode(amount);
        sendRequest(dst, amount, Nil.ASYNC_REQUEST_MIN_GAS, context, "", callback);
    }

    function callback(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable onlyResponse {
        uint amount = abi.decode(context, (uint));
        if (!success) {
            exampleBalances[Nil.msgSender()] += amount;
        }
    }

    //endGoodCheckEffectsInteraction
}
