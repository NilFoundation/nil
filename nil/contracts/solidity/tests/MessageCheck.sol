// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.9;

import "../../../../smart-contracts/contracts/Nil.sol";

contract MessageCheck is NilBase {

    function externalFunc() onlyExternal public {
    }

    function internalFunc() onlyInternal public {
    }

    // Fail: we call external method by sync call, which is considered as internal
    function callExternal(address addr) onlyExternal public {
        MessageCheck(addr).externalFunc();
    }

    // Ok: we call internal method by sync call
    function callInternal(address addr) onlyExternal public {
        MessageCheck(addr).internalFunc();
    }

    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }
}
