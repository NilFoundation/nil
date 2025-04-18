// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "./Nil.sol";
import "./console.sol";
import "./NilTokenManager.sol";

contract Relayer {

    function sendTx(
        address to,
        address refundTo,
        uint feeCredit,
        uint8 forwardKind,
        uint value,
        Nil.Token[] memory tokens,
        bytes memory callData
    ) public payable {
        console.log("sendTx: tokens=%_", tokens.length);
        NilTokenManager(Nil.getTokenManagerAddress()).deductForRelay(msg.sender, to, tokens);
        bytes memory data = abi.encodeWithSelector(this.receiveTx.selector, msg.sender, to, value, tokens, callData);
        console.log("sendTx: packed");
        __Precompile__(Nil.ASYNC_CALL).precompileAsyncCall{value: value}(
            false,
            forwardKind,
            Nil.getRelayerAddress(Nil.getShardId(to)),
            refundTo,
            refundTo,
            feeCredit,
            tokens,
            data);
        console.log("sendTx: finish");
    }

    function receiveTx(
        address from,
        address to,
        uint value,
        Nil.Token[] memory tokens,
        bytes memory callData
    ) public payable {
        console.log("receiveTx: tokens=%_", tokens.length);

        NilTokenManager(Nil.getTokenManagerAddress()).creditForRelay(to, tokens);
        (bool success, bytes memory returnData) = to.call{value: value}(callData);
        NilTokenManager(Nil.getTokenManagerAddress()).resetTxTokens();

        if (!success) {
            printRevertData("receiveTx call failed", returnData);
            NilTokenManager(Nil.getTokenManagerAddress()).deductForRelay(to, address(this), tokens);
            bytes memory data = abi.encodeWithSelector(this.receiveTxBounce.selector, from, value, tokens, returnData);
            __Precompile__(Nil.ASYNC_CALL).precompileAsyncCall{value: value}(
                false,
                Nil.FORWARD_REMAINING,
                Nil.getRelayerAddress(Nil.getShardId(from)),
                from,
                from,
                0,
                tokens,
                data);
        }
    }

    function receiveTxBounce(
        address to,
        uint value,
        Nil.Token[] memory tokens,
        bytes memory callData
    ) public payable {
        printRevertData("Bounce tx", callData);
        NilTokenManager(Nil.getTokenManagerAddress()).creditForRelay(to, tokens);
        bytes memory data = abi.encodeWithSignature("bounce(bytes)", callData);
        (bool success, bytes memory returnData) = to.call{value: value}(data);
        if (!success) {
            printRevertData("Bounce call failed", returnData);
        }
    }

    function printRevertData(string memory str, bytes memory returnData) internal pure {
        if (returnData.length > 68) {
            assembly {
                returnData := add(returnData, 0x04)
            }
            string memory reason = abi.decode(returnData, (string));
            console.log("%_: %_", str, reason);
        } else {
            console.log("%_: <no revert reason>", str);
        }
    }
}
