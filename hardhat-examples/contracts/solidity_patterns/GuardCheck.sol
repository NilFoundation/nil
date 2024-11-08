// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract GuardCheckParent is NilBase {

    uint public exampleBalance;

    function topUpBalance(uint amount) public {
        exampleBalance += amount;
    }

    function guardCheck(address dst, uint amount, uint restBalance) public {
        // GuardCheck require:
        require(exampleBalance >= amount);

        // GuardCheck revert:
        if (amount < 1000) {
            revert();
        }

        uint exampleBalanceBeforeAsyncCall = exampleBalance;
        exampleBalance -= amount;

        Nil.asyncCall(dst, msg.sender, msg.sender, 100000, 0, false, 0, abi.encodeWithSignature("childFunc()"));

        // GuardCheck assert:
        assert(restBalance == exampleBalanceBeforeAsyncCall - amount);
    }
}

contract GuardCheckChild is NilBase {
    bool public executed = false;

    function childFunc() public {
        executed = true;
    }
}
