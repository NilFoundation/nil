// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.9;

import "../lib/Nil.sol";

contract PrecompilesTest is NilBase {

    function testAsyncCall(
        address dst,
        address refundTo,
        address bounceTo,
        uint feeCredit,
        uint8 forwardKind,
        uint value,
        bytes memory callData) public {
        Nil.asyncCall(dst, refundTo, bounceTo, feeCredit, forwardKind, value, callData);
    }

    function testSendRawMsg(
        bytes memory callData) public {
        Nil.sendMessage(callData);
    }

    function testCurrencyBalance(address addr, CurrencyId currencyId) public view returns(uint) {
        return Nil.currencyBalance(addr, currencyId);
    }
}
