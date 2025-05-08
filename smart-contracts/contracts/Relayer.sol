// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./NilTokenManager.sol";
import "../system/console.sol";

/**
 * @title Relayer
 * @dev This contract facilitates relaying transactions, handling responses, and managing token credits.
 */
contract Relayer {

    struct Transaction {
        address from;
        address to;
        address refundTo;
        address bounceTo;
        uint256 value;
        Nil.Token[] tokens;
        uint8 forwardKind;
        uint256 feeCredit;
        uint256 responseFeeCredit;
        bytes callData;
    }

    Transaction[] private currentTransactions;
    uint8[] private txsForwardedRemaining;
    uint8[] private txsForwardedPercentage;
    uint8[] private txsForwardedValue;
    uint32 private numTxsWithForwardGas;
    bool private forwardingInitialized;

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

    function startAsync() public payable {
        require(!forwardingInitialized, "Relayer already initialized");
        console.log("Relayer start");
        numTxsWithForwardGas = 0;
        delete currentTransactions;
        forwardingInitialized = true;
    }

    function finalizeAsync(uint gas) public payable {
        console.log("Relayer finalize: gas=%_, num=%_", gas, numTxsWithForwardGas);
        _forwardGas(gas);
        _resetForwarding();
    }

    function _forwardGas(uint gas) internal {
//        uint numTxsWithForwardGas = txsForwardedPercentage.length + txsForwardedRemaining.length;
        uint feeCredit = gas * tx.gasprice;
        console.log("_forwardGas: gas=%_, num=%_", gas, numTxsWithForwardGas);

        if (numTxsWithForwardGas == 0) {
            return;
        }

        for (uint256 i = 0; i < txsForwardedValue.length; i++) {
            uint256 index = txsForwardedValue[i];
            Transaction storage txn = currentTransactions[index];
            if (txn.forwardKind == Nil.FORWARD_VALUE) {
                console.log("_forwardGas value: to=%_, fee=%_", txn.to, txn.feeCredit);
                require(feeCredit >= txn.feeCredit, "forwardGas: not enough feeCredit for ForwardValue");
                feeCredit -= txn.feeCredit;
            }
        }

        uint percentageTotal = 0;
        uint baseFeeCredit = feeCredit;
        for (uint256 i = 0; i < txsForwardedPercentage.length; i++) {
            uint256 index = txsForwardedPercentage[i];
            Transaction storage txn = currentTransactions[index];
            require(txn.forwardKind == Nil.FORWARD_PERCENTAGE, "forwardGas: invalid percentage forwarding");

            percentageTotal += txn.feeCredit;
            if (percentageTotal > 100) {
                revert("forwardGas: total percentage is greater than 100");
            }

            txn.feeCredit = (txn.feeCredit * baseFeeCredit) / 100;

            if (feeCredit < txn.feeCredit) {
                txn.feeCredit = feeCredit;
                feeCredit = 0;
            } else {
                feeCredit -= txn.feeCredit;
            }

            console.log("_forwardGas percentage: fee=%_", txn.feeCredit);
        }

        if (txsForwardedRemaining.length != 0) {
            if (feeCredit == 0) {
                revert("forwardGas: not enough feeCredit for ForwardRemaining");
            }
            uint feeCreditForward = feeCredit / txsForwardedRemaining.length;
            feeCredit = 0;
            for (uint256 i = 0; i < txsForwardedRemaining.length; i++) {
                uint256 index = txsForwardedRemaining[i];
                Transaction storage txn = currentTransactions[index];

                console.log("_forwardGas remaining: to=%_, fee=%_", txn.to, feeCreditForward);

                require(txn.forwardKind == Nil.FORWARD_REMAINING);
                txn.feeCredit = feeCreditForward;
            }
        }

        for (uint256 i = 0; i < currentTransactions.length; i++) {
            Transaction storage txn = currentTransactions[i];
            console.log("_forwardGas SEND: to=%_, fee=%_", txn.to, txn.feeCredit);
            __Precompile__(Nil.ASYNC_CALL).precompileAsyncCall{value: txn.value}(
                false,
                txn.forwardKind,
                txn.to,
                txn.refundTo,
                txn.bounceTo,
                txn.feeCredit,
                txn.tokens,
                txn.callData,
                0, 0);
        }

        if (feeCredit != 0) {
            console.log("_forwardGas: return fee %_ to %_", feeCredit, msg.sender);
            payable(msg.sender).transfer(feeCredit);
        }
    }

    function _resetForwarding() internal {
        numTxsWithForwardGas = 0;
        delete currentTransactions;
        delete txsForwardedRemaining;
        delete txsForwardedPercentage;
        delete txsForwardedValue;
        forwardingInitialized = false;
    }

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
        console.log("sendTx: to=%_, from=%_", to, msg.sender);

        require(forwardingInitialized, "Relayer not initialized");

        if (forwardKind == Nil.FORWARD_REMAINING) {
            txsForwardedRemaining.push(uint8(currentTransactions.length));
            numTxsWithForwardGas++;
            console.log("sendTx FORWARD_REMAINING: num=%_", numTxsWithForwardGas);
        } else if (forwardKind == Nil.FORWARD_PERCENTAGE) {
            txsForwardedPercentage.push(uint8(currentTransactions.length));
            numTxsWithForwardGas++;
            console.log("sendTx FORWARD_PERCENTAGE: num=%_", numTxsWithForwardGas);
        } else if (forwardKind == Nil.FORWARD_VALUE) {
            txsForwardedValue.push(uint8(currentTransactions.length));
            numTxsWithForwardGas++;
            console.log("sendTx FORWARD_VALUE: num=%_", numTxsWithForwardGas);
        } else if (forwardKind == Nil.FORWARD_NONE) {
            numTxsWithForwardGas++;
        } else {
            revert("sendTx: invalid forwardKind");
        }

        uint256 responseFeeCredit;
        if (requestId != 0) {
            require(responseGas > 0, "sendTx: responseGas must be greater than 0");
            responseFeeCredit = responseGas * Nil.getGasPrice(address(this));
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

        currentTransactions.push(
            Transaction(msg.sender, Nil.getRelayerAddress(Nil.getShardId(to)), refundTo, bounceTo, value, tokens, forwardKind, feeCredit, responseFeeCredit, data)
        );
        console.log("sendTx pushed: numtxs=%_", currentTransactions.length);
    }

    function calculateAsyncGas(bool request, bool success) internal view returns(uint) {
        uint asyncGas;
        console.log("receiveTx: numTxsWithForwardGas=%_", numTxsWithForwardGas);
        if (numTxsWithForwardGas != 0) {
            uint gasToFinish = 1000;
            uint gasForTxForward = 2000;
            uint gasForResponse = 10_000;
            uint gasForBounce = 10_000;

            uint requiredGasForFinish = gasToFinish;

            if (request) {
                requiredGasForFinish += gasForResponse;
            } else if (!success) {
                requiredGasForFinish += gasForBounce;
            }
            if (success) {
                requiredGasForFinish = numTxsWithForwardGas * gasForTxForward;
            }
            if (gasleft() < requiredGasForFinish) {
                success = false;
            } else {
                asyncGas = gasleft() - requiredGasForFinish;
            }
        }
        return asyncGas;
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
        address from,
        address to,
        address bounceTo,
        uint value,
        Nil.Token[] memory tokens,
        bytes memory callData,
        uint256 requestId,
        uint responseFeeCredit
    ) public payable returns(bytes memory) {
        console.log("receiveTx: gas=%_, to=%_, from=%_", gasleft(), to, from);

//        startAsync();
        NilTokenManager(Nil.getTokenManagerAddress()).creditForRelay(to, tokens);

        (bool success, bytes memory returnData) = to.call{value: value}(callData);

        NilTokenManager(Nil.getTokenManagerAddress()).resetTxTokens();

        console.log("receiveTx: success=%_, gasleft=%_", success, gasleft());

//        finalizeAsync(calculateAsyncGas(requestId != 0, success));

        if (requestId != 0) {
            printRevertData("receiveTx request", returnData);
            uint256 returnValue = 0;
            if (!success) {
                returnValue = value;
            }
            bytes memory data = abi.encodeWithSelector(
                this.receiveTxResponse.selector, to, from, returnValue, success, returnData, requestId, responseFeeCredit);
            __Precompile__(Nil.ASYNC_CALL).precompileAsyncCall(
                false,
                Nil.FORWARD_REMAINING,
                Nil.getRelayerAddress(Nil.getShardId(from)),
                from,
                from,
                0,
                new Nil.Token[](0),
                data,
                0,
                0
            );
            return bytes("");
        }

        if (!success) {
            printRevertData("receiveTx call failed", returnData);

            emit CallFailed(from, to, value, tokens, callData);

            NilTokenManager(Nil.getTokenManagerAddress()).deductForRelay(to, address(this), tokens);
            bytes memory data = abi.encodeWithSelector(this.receiveTxBounce.selector, bounceTo, value, tokens, returnData);
            __Precompile__(Nil.ASYNC_CALL).precompileAsyncCall{value: value}(
                false,
                Nil.FORWARD_REMAINING,
                Nil.getRelayerAddress(Nil.getShardId(from)),
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
        uint gas = responseFeeCredit / Nil.getGasPrice(address(this));
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
