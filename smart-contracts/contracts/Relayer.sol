// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "./Nil.sol";
import "./TokenManager.sol";


contract Relayer {

    function sendTx(
        address dst,
        address refundTo,
        uint feeCredit,
        uint8 forwardKind,
        uint value,
        Nil.Token[] memory tokens,
        bytes memory callData
    ) public {
        for (uint i = 0; i < tokens.length; i++) {
            TokenManager(Nil.getTokenManagerAddress()).deduct(msg.sender, TokenId.unwrap(tokens[i].id), tokens[i].amount);
        }
        bytes memory data = abi.encodeWithSelector(this.__receiveTx.selector, dst, refundTo, feeCredit, 0, value, tokens, callData);
        __Precompile__(Nil.ASYNC_CALL).precompileAsyncCall{value: value}(
            false,
            forwardKind,
            Nil.getRelayerAddress(Nil.getShardId(dst)),
            refundTo,
            refundTo,
            feeCredit,
            tokens,
            data);
    }

    function __receiveTx(
        address dst,
        address /*refundTo*/,
        uint /*feeCredit*/,
        uint8 /*forwardKind*/,
        uint value,
        Nil.Token[] memory tokens,
        bytes memory callData
    ) public {
        for (uint i = 0; i < tokens.length; i++) {
            TokenManager(Nil.getTokenManagerAddress()).credit(dst, TokenId.unwrap(tokens[i].id), tokens[i].amount);
        }
        (bool success, bytes memory data) = dst.call{value: value}(callData);
        require(success, "Call failed");
    }
}
