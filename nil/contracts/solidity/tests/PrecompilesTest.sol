// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.9;

import "../lib/Nil.sol";

contract PrecompilesTest is NilBase {
    function testAsyncCall(
        uint256 shardIdDst,
        address dst,
        address refundTo,
        address bounceTo,
        uint feeCredit,
        uint8 forwardKind,
        uint value,
        bytes memory callData
    ) public {
        Nil.asyncCall(
            shardIdDst,
            dst,
            refundTo,
            bounceTo,
            feeCredit,
            forwardKind,
            value,
            callData
        );
    }

    function testTokenBalance(
        uint256 shardIdDst,
        address addr,
        TokenId tokenId
    ) public view returns (uint) {
        return Nil.tokenBalance(shardIdDst, addr, tokenId);
    }
}
