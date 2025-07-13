// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./NilTokenManager.sol";
//import "./../system/console.sol";

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
        bool deploy;
        uint256 salt;
        bytes callData;
    }
    uint constant gasForBounce = 200_000;

    // List of transactions to be sent
    Transaction[] private currentTransactions;
    // Indexes of transactions that are forwarded with FORWARD_REMAINING
    uint8[] private txsForwardedRemaining;
    // Indexes of transactions that are forwarded with FORWARD_PERCENTAGE
    uint8[] private txsForwardedPercentage;
    // Indexes of transactions that are forwarded with FORWARD_VALUE
    uint8[] private txsForwardedValue;
    // Store map of refunds that were failed to send
    mapping(address => uint256) private pendingRefund;

    // The address that initiated async calls
    address private initiator;
    // The number of nested async modifier runs
    int private asyncModifierNum;

    address private sender;

    enum CallFailKind {
        REGULAR,
        DEPLOY,
        RESPONSE
    }
    /**
     * @dev Emitted when a call fails to execute.
     * @param from The address that initiated the call.
     * @param to The target address of the call.
     * @param value The amount of Ether sent with the call.
     * @param tokens The tokens involved in the call.
     * @param callData The calldata of the call.
     */
    event CallFailed(
        CallFailKind indexed callKind,
        address indexed from,
        address indexed to,
        uint256 value,
        Nil.Token[] tokens,
        bytes callData
    );

    function txSender() public view returns (address) {
        return sender;
    }

    /**
     * @dev Initialize Relayer for further cross-shard transactions. Should be always called before any async call.
     */
    function startAsync() public payable {
        require (initiator == address(0) || msg.sender == initiator, "startAsync: async send from different addresses");
        initiator = msg.sender;
        if (asyncModifierNum == 0) {
            delete currentTransactions;
        }
        asyncModifierNum++;
    }

    /**
     * @dev Finalize all async calls by forwarding gas to all transactions.
     * @param gas The amount of gas available for forwarding.
     */
    function finalizeAsync(uint gas) public payable {
        asyncModifierNum--;
        //console.log("finalizeAsync: asyncModifierNum=%_", uint(asyncModifierNum));
        require (asyncModifierNum >= 0, "finalizeAsync: wrong async modifier run");
        if (asyncModifierNum != 0) {
            return;
        }
        //console.log("finalizeAsync: start");

        _forwardGas(gas);
        _resetForwarding();
        //console.log("finalizeAsync finish: gasleft=%_", gasleft());
    }

    /**
     * @dev Forwards gas to all transactions.
     * @param gas The amount of gas available for forwarding.
     */
    function _forwardGas(uint gas) internal {
        uint feeCredit = gas * tx.gasprice;

        for (uint256 i = 0; i < txsForwardedValue.length; i++) {
            uint256 index = txsForwardedValue[i];
            Transaction storage txn = currentTransactions[index];
            if (txn.forwardKind == Nil.FORWARD_VALUE) {
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

                require(txn.forwardKind == Nil.FORWARD_REMAINING);
                txn.feeCredit = feeCreditForward;
            }
        }

        for (uint256 i = 0; i < currentTransactions.length; i++) {
            Transaction storage txn = currentTransactions[i];

            if (txn.responseFeeCredit != 0) {
                require(txn.feeCredit >= txn.responseFeeCredit, "sendTx: feeCredit must be greater than responseFeeCredit");
                txn.feeCredit -= txn.responseFeeCredit;
            }

            bool s = __Precompile__(Nil.ASYNC_CALL).precompileAsyncCall{value: txn.value}(
                false,
                txn.forwardKind,
                txn.to,
                txn.refundTo,
                txn.bounceTo,
                txn.feeCredit,
                txn.tokens,
                txn.callData,
                0, 0);
            if (!s) {
                //console.log("forwardGas: 5");
            }
        }

        if (feeCredit != 0) {
            //console.log("forwardGas: returning feeCredit=%_ to=%_", feeCredit, msg.sender);
            bytes memory data = abi.encodeWithSignature("nilReceive()");
            (bool success,) = payable(msg.sender).call{value: feeCredit}(data);
            if (!success) {
                revert("forwardGas: failed to return feeCredit(probably nilReceive is not implemented)");
            }
        }
    }

    /**
     * @dev Resets the forwarding state.
     */
    function _resetForwarding() internal {
        initiator = address(0);
        delete currentTransactions;
        delete txsForwardedRemaining;
        delete txsForwardedPercentage;
        delete txsForwardedValue;
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
        uint256 feeCredit,
        uint8 forwardKind,
        uint256 value,
        Nil.Token[] memory tokens,
        bytes memory callData,
        uint256 requestId,
        uint256 responseGas
    ) public payable {
        //console.log("sendTx: to=%_, from=%_", to, msg.sender);

        require(asyncModifierNum != 0, "Relayer not initialized");

        if (forwardKind == Nil.FORWARD_REMAINING) {
            txsForwardedRemaining.push(uint8(currentTransactions.length));
        } else if (forwardKind == Nil.FORWARD_PERCENTAGE) {
            txsForwardedPercentage.push(uint8(currentTransactions.length));
        } else if (forwardKind == Nil.FORWARD_VALUE) {
            txsForwardedValue.push(uint8(currentTransactions.length));
        } else if (forwardKind == Nil.FORWARD_NONE) {
        } else {
            revert("sendTx: invalid forwardKind");
        }

        if (requestId != 0) {
            require(responseGas > 0, "sendTx: responseGas must be greater than 0");
        }

        if (refundTo == address(0)) {
            refundTo = msg.sender;
        }
        if (bounceTo == address(0)) {
            bounceTo = msg.sender;
        }

        uint256 responseFeeCredit = responseGas * tx.gasprice;
        NilTokenManager(Nil.getTokenManagerAddress()).deductForRelay(msg.sender, to, tokens);
        bytes memory data = abi.encodeWithSelector(
            this.receiveTx.selector, msg.sender, to, bounceTo, value, tokens, callData, requestId, responseFeeCredit);

        currentTransactions.push(
            Transaction(
                msg.sender,
                Nil.getRelayerAddress(Nil.getShardId(to)),
                refundTo,
                bounceTo,
                value,
                tokens,
                forwardKind,
                feeCredit,
                responseFeeCredit,
                false,
                0,
                data)
        );
        //console.log("sendTx done: numtxs=%_", currentTransactions.length);
    }

    function sendTxDeploy(
        address to,
        address refundTo,
        address bounceTo,
        uint256 feeCredit,
        uint8 forwardKind,
        uint256 value,
        uint256 salt,
        bytes memory callData
    ) public payable {
        //console.log("sendTxDeploy1: gasleft=%_, fee=%_, to=%_, value=%_", gasleft(), feeCredit, to, value);

        require(asyncModifierNum != 0, "Relayer not initialized");

        if (forwardKind == Nil.FORWARD_REMAINING) {
            txsForwardedRemaining.push(uint8(currentTransactions.length));
        } else if (forwardKind == Nil.FORWARD_PERCENTAGE) {
            txsForwardedPercentage.push(uint8(currentTransactions.length));
        } else if (forwardKind == Nil.FORWARD_VALUE) {
            txsForwardedValue.push(uint8(currentTransactions.length));
        } else if (forwardKind == Nil.FORWARD_NONE) {
        } else {
            revert("sendTxDeploy: invalid forwardKind");
        }

        if (refundTo == address(0)) {
            refundTo = msg.sender;
        }
        if (bounceTo == address(0)) {
            bounceTo = msg.sender;
        }

        bytes memory data = abi.encodeWithSelector(
            this.receiveTxDeploy.selector, msg.sender, bounceTo, value, salt, callData);

        //console.log("sendTxDeploy finish: gasleft=%_, to=%_", gasleft(), to);

        currentTransactions.push(
            Transaction(
                msg.sender,
                Nil.getRelayerAddress(Nil.getShardId(to)),
                refundTo,
                bounceTo,
                value,
                new Nil.Token[](0),
                forwardKind,
                feeCredit,
                0,
                true,
                salt,
                data)
        );
        //console.log("sendTxDeploy finish 2: gasleft=%_", gasleft());

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
        //console.log("receiveTx: gas=%_, to=%_, value=%_, code=%_", gasleft(), to, value, to.code.length);

        sender = from;

        require(value != 0 || tokens.length != 0 || to.code.length != 0, "receiveTx: target contract must have code");

        NilTokenManager(Nil.getTokenManagerAddress()).creditForRelay(to, tokens);

        uint gasForCall = calculateGasForTargetCall(requestId != 0);

        //console.log("receiveTx call: gas=%_, addr=%_", gasForCall, to);

        (bool success, bytes memory returnData) = to.call{value: value, gas: gasForCall}(callData);

        //console.log("receiveTx: success=%_, gasleft=%_", success, gasleft());

        NilTokenManager(Nil.getTokenManagerAddress()).resetTxTokens();

        if (requestId != 0) {
            uint256 returnValue = 0;
            if (!success) {
                printRevertData("receiveTx call failed", returnData);
                returnValue = value;
            }
            bytes memory data = abi.encodeWithSelector(
                this.receiveTxResponse.selector, to, from, returnValue, success, returnData, requestId);
            __Precompile__(Nil.ASYNC_CALL).precompileAsyncCall(
                false,
                Nil.FORWARD_REMAINING,
                Nil.getRelayerAddress(Nil.getShardId(from)),
                from,
                from,
                responseFeeCredit,
                new Nil.Token[](0),
                data,
                0,
                0
            );
            return bytes("");
        }

        if (!success) {
            printRevertData("receiveTx call failed", returnData);

            if (bounceTo == address(0)) {
                bounceTo = from;
            }

            emit CallFailed(CallFailKind.REGULAR, from, to, value, tokens, callData);

            NilTokenManager(Nil.getTokenManagerAddress()).deductForRelay(to, address(this), tokens);

            bytes memory data = abi.encodeWithSelector(this.receiveTxBounce.selector, bounceTo, value, tokens, returnData);
            uint feeForBounce = gasForBounce * tx.gasprice;
            __Precompile__(Nil.ASYNC_CALL).precompileAsyncCall{value: value /*+ feeForBounce*/}(
                false,
                Nil.FORWARD_REMAINING,
                Nil.getRelayerAddress(Nil.getShardId(bounceTo)),
                address(this),
                address(this),
                feeForBounce,
                tokens,
                data,
                0,
                0);
            return bytes("");
        }

        return returnData;
    }

    /**
     * @dev Process deploy transaction.
     * @param from The address that initiated the transaction.
     * @param bounceTo The address to bounce the transaction to in case of failure.
     * @param value The value sent with the transaction.
     * @param salt The salt used for creating the contract address.
     * @param code The bytecode of the contract to deploy.
     * @return The return data from the transaction.
     */
    function receiveTxDeploy(
        address from,
        address bounceTo,
        uint256 value,
        uint256 salt,
        bytes memory code
    ) public payable returns(bytes memory) {
        //console.log("receiveTxDeploy: size=%_, salt=%_, value=%_", code.length, salt, value);

        sender = from;

        address addr;
        assembly {
            addr := create2(value, add(code, 0x20), mload(code), salt)
        }
        bool success = addr != address(0);

        //console.log("receiveTxDeploy: addr=%_, balance=%_", addr, address(addr).balance);

        if (!success) {
            emit CallFailed(CallFailKind.DEPLOY, from, address(0), value, new Nil.Token[](0), bytes(""));

            if (bounceTo == address(0)) {
                bounceTo = from;
            }

            bytes memory data = abi.encodeWithSelector(this.receiveTxBounce.selector, bounceTo, value, new Nil.Token[](0), bytes(""));
            // At the moment Relayer pays for the bounce message. TODO: payment should be withheld from the sender
            uint feeForBounce = gasForBounce * tx.gasprice;
            __Precompile__(Nil.ASYNC_CALL).precompileAsyncCall{value: value}(
                false,
                Nil.FORWARD_REMAINING,
                Nil.getRelayerAddress(Nil.getShardId(bounceTo)),
                address(this),
                address(this),
                feeForBounce,
                new Nil.Token[](0),
                data,
                0,
                0);
        }

        return bytes("");
    }

    /**
     * @dev Handles the response of a transaction.
     * @param from The address that initiated the transaction.
     * @param to The target address of the transaction.
     * @param value The amount of Ether sent with the transaction.
     * @param success Indicates whether the transaction was successful.
     * @param response The response data.
     * @param requestId The ID of the request.
     */
    function receiveTxResponse(
        address from,
        address to,
        uint256 value,
        bool success,
        bytes memory response,
        uint256 requestId
    ) public payable {
        bytes memory data = abi.encodeWithSignature("onFallback(uint256,bool,bytes)", requestId, success, response);
        (bool s, bytes memory returnData) = to.call{value: value}(data);
        if (!s) {
            printRevertData("Response call failed", returnData);
            emit CallFailed(CallFailKind.RESPONSE, from, to, value, new Nil.Token[](0), response);
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
            // Save the value for the future refund. TODO: support postponed refunding
            pendingRefund[to] += value;
            printRevertData("Bounce call failed", returnData);
        }
    }

    /**
     * @dev Calculates the gas required for a target call.
     * @param request Indicates whether the call is a request.
     * @return The amount of gas required for the call of the target contract.
     */
    function calculateGasForTargetCall(bool request) internal view returns(uint) {
        uint gasForResponse = 50_000;
        uint gasForFinish = 50_000;

        if (request) {
            gasForFinish += gasForResponse;
        }

        if (gasleft() < gasForFinish) {
            gasForFinish = 0;
        } else {
            gasForFinish = gasleft() - gasForFinish;
        }
        return gasForFinish;
    }

//    function printRevertData(string memory str, bytes memory returnData) internal pure {
//        if (returnData.length > 68) {
//            assembly {
//                returnData := add(returnData, 0x04)
//            }
//            string memory reason = abi.decode(returnData, (string));
//            //console.log("%_: %_", str, reason);
//        } else {
//            //console.log("%_: <no revert reason>", str);
//        }
//    }

    function printRevertData(string memory /*str*/, bytes memory /*returnData*/) internal pure {
    }

    function nilReceive() payable external {}
}
