// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./NilTokenManager.sol";

/**
 * @title Relayer
 * @dev This contract facilitates relaying transactions, handling responses, and managing token credits.
 */
contract Relayer {
    uint256 shardId;

    constructor(uint256 _shardId) {
        shardId = _shardId;
    }

    function GetShardId() public view returns(uint256) {
        return shardId;
    }

    /**
     * @dev Emitted when a response fails to execute.
     * @param from The address that initiated the response.
     * @param to The target address of the response.
     * @param success Indicates whether the response was successful.
     * @param response The response data.
     * @param requestId The ID of the request associated with the response.
     * @param responseFeeCredit The fee credit allocated for the response.
     */
    event ResponseFailed(
        address indexed from,
        address indexed to,
        bool success,
        bytes response,
        uint256 requestId,
        uint256 responseFeeCredit
    );

    /**
     * @dev Emitted when a call fails to execute.
     * @param from The address that initiated the call.
     * @param to The target address of the call.
     * @param value The amount of Ether sent with the call.
     * @param tokens The tokens involved in the call.
     * @param callData The calldata of the call.
     */
    event CallFailed(
        address indexed from,
        address indexed to,
        uint256 value,
        Nil.Token[] tokens,
        bytes callData
    );

    /**
     * @dev Sends a transaction to a target address with optional refund and bounce handling.
     * @param to The target address.
     * @param refundTo The address to refund in case of failure.
     * @param bounceTo The address to bounce the transaction to in case of failure.
     * @param feeCredit The fee credit for the transaction.
     * @param forwardKind The forwarding type.
     * @param value The amount of Ether to send.
     * @param tokens The tokens to relay.
     * @param callData The calldata for the transaction.
     * @param requestId The ID of the request.
     * @param responseGas The gas allocated for the response.
     */
    function sendTx(
        uint256 shardIdTo,
        address to,
        address refundTo,
        address bounceTo,
        uint feeCredit,
        uint8 forwardKind,
        uint value,
        Nil.Token[] memory tokens,
        bytes memory callData,
        uint256 requestId,
        uint responseGas
    ) public payable {
        uint256 responseFeeCredit;
        if (requestId != 0) {
            require(responseGas > 0, "sendTx: responseGas must be greater than 0");
            responseFeeCredit = responseGas * Nil.getGasPrice(shardId);
            require(feeCredit >= responseFeeCredit, "sendTx: feeCredit must be greater than responseFeeCredit");
            feeCredit -= responseFeeCredit;
        }

        if (refundTo == address(0)) {
            refundTo = msg.sender;
        }
        if (bounceTo == address(0)) {
            bounceTo = msg.sender;
        }

        NilTokenManager(Nil.getTokenManagerAddress()).deductForRelay(msg.sender, to, tokens);
        bytes memory data = abi.encodeWithSelector(
            this.receiveTx.selector, msg.sender, to, bounceTo, value, tokens, callData, requestId, responseFeeCredit);

        __Precompile__(Nil.ASYNC_CALL).precompileAsyncCall{value: value}(
            false,
            forwardKind,
            Nil.getRelayerAddress(shardIdTo),
            refundTo,
            bounceTo,
            feeCredit,
            tokens,
            data,
            0,
            0);
    }

    /**
     * @dev Handles the receipt of a transaction.
     * @param from The address that initiated the transaction.
     * @param to The target address of the transaction.
     * @param value The amount of Ether sent with the transaction.
     * @param tokens The tokens involved in the transaction.
     * @param callData The calldata of the transaction.
     * @param requestId The ID of the request.
     * @param responseFeeCredit The fee credit allocated for the response.
     * @return The return data from the transaction.
     */
    function receiveTx(
        uint256 shardIdFrom,
        address from,
        address to,
        address bounceTo,
        uint value,
        Nil.Token[] memory tokens,
        bytes memory callData,
        uint256 requestId,
        uint responseFeeCredit
    ) public payable returns(bytes memory) {
        NilTokenManager(Nil.getTokenManagerAddress()).creditForRelay(to, tokens);
        (bool success, bytes memory returnData) = to.call{value: value}(callData);
        NilTokenManager(Nil.getTokenManagerAddress()).resetTxTokens();

        if (requestId != 0) {
            uint256 returnValue = 0;
            if (!success) {
                returnValue = value;
            }
            bytes memory data = abi.encodeWithSelector(
                this.receiveTxResponse.selector, to, from, returnValue, success, returnData, requestId, responseFeeCredit);
            __Precompile__(Nil.ASYNC_CALL).precompileAsyncCall(
                false,
                Nil.FORWARD_REMAINING,
                Nil.getRelayerAddress(shardIdFrom),
                from,
                from,
                0,
                new Nil.Token[](0),
                data,
                0,
                0
            );
            return bytes("");
        } else if (!success) {
            printRevertData("receiveTx call failed", returnData);

            emit CallFailed(from, to, value, tokens, callData);

            NilTokenManager(Nil.getTokenManagerAddress()).deductForRelay(to, address(this), tokens);
            bytes memory data = abi.encodeWithSelector(this.receiveTxBounce.selector, bounceTo, value, tokens, returnData);
            __Precompile__(Nil.ASYNC_CALL).precompileAsyncCall{value: value}(
                false,
                Nil.FORWARD_REMAINING,
                Nil.getRelayerAddress(shardIdFrom),
                from,
                from,
                0,
                tokens,
                data,
                0,
                0);
            return bytes("");
        }
        return returnData;
    }

    /**
     * @dev Handles the response of a transaction.
     * @param from The address that initiated the transaction.
     * @param to The target address of the transaction.
     * @param value The amount of Ether sent with the transaction.
     * @param success Indicates whether the transaction was successful.
     * @param response The response data.
     * @param requestId The ID of the request.
     * @param responseFeeCredit The fee credit allocated for the response.
     */
    function receiveTxResponse(
        address from,
        address to,
        uint256 value,
        bool success,
        bytes memory response,
        uint256 requestId,
        uint256 responseFeeCredit
    ) public payable {
        uint gas = responseFeeCredit / Nil.getGasPrice(shardId);
        bytes memory data = abi.encodeWithSignature("onFallback(uint256,bool,bytes)", requestId, success, response);
        (bool s, ) = to.call{gas: gas, value: value}(data);
        if (!s) {
            emit ResponseFailed(to, from, success, response, requestId, responseFeeCredit);
        }
    }

    /**
     * @dev Handles the bounce of a failed transaction.
     * @param to The target address of the bounce.
     * @param value The amount of Ether sent with the bounce.
     * @param tokens The tokens involved in the bounce.
     * @param callData The calldata of the bounce.
     */
    function receiveTxBounce(
        address to,
        uint value,
        Nil.Token[] memory tokens,
        bytes memory callData
    ) public payable {
        printRevertData("Bounce tx", callData);
        NilTokenManager(Nil.getTokenManagerAddress()).creditForRelay(to, tokens);
        NilTokenManager(Nil.getTokenManagerAddress()).resetTxTokens();

        bytes memory data = abi.encodeWithSignature("bounce(bytes)", callData);
        (bool success, bytes memory returnData) = to.call{value: value}(data);
        if (!success) {
            printRevertData("Bounce call failed", returnData);
        }
    }

    function printRevertData(string memory /*str*/, bytes memory /*returnData*/) internal pure {
//        if (returnData.length > 68) {
//            assembly {
//                returnData := add(returnData, 0x04)
//            }
//            string memory reason = abi.decode(returnData, (string));
//            console.log("%_: %_", str, reason);
//        } else {
//            console.log("%_: <no revert reason>", str);
//        }
    }
}