// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./NilTokenManager.sol";
import "./IterableMapping.sol";

// Dirty hack to run the tests with lesser number of shards
uint32 constant _N_SHARDS = 6;

/**
 * @title Relayer
 * @dev This contract facilitates relaying transactions, handling responses, and managing token credits.
 * It contains a message queue for asynchronous cross-shard communication.
 */
contract Relayer {
    using IterableMapping for IterableMapping.Map;

    // Structure for cross-shard messages
    struct Message {
        uint64 id;                   // Unique message identifier
        address from;                // Source address
        address to;                  // Destination address
        address refundTo;            // Refund address
        address bounceTo;            // Bounce address
        uint256 value;               // Value to transfer
        Nil.Token[] tokens;          // Tokens to transfer
        uint8 forwardKind;           // Forwarding kind
        uint256 feeCredit;           // Fee credit
        bytes data;                  // Call data
        uint256 requestId;           // For response tracking
        uint256 responseFeeCredit;   // Fee credit allocated for response
        bool isDeploy;               // Whether this is a deploy message
        uint256 salt;                // For deploy messages
    }
    
    // Struct to group message parameters to reduce stack depth
    struct MessageParams {
        address from;
        address to;
        address refundTo;
        address bounceTo;
        uint256 value;
        uint8 forwardKind;
        uint256 feeCredit;
        uint256 requestId;
        uint256 responseFeeCredit;
        bool isDeploy;
        uint256 salt;
    }
    
    // Message queue data
    mapping(uint32 => Message[]) messages;
    uint64[_N_SHARDS] private inMsgCount;
    uint64[_N_SHARDS] private outMsgCount;
    uint64[_N_SHARDS] private currentBlockNumber;

    struct MessageRef {
        uint32 shardId;
        uint32 messageIndex;
    }
    
    // For async call management
    MessageRef[] private msgsForwardedRemaining;
    MessageRef[] private msgsForwardedPercentage;
    MessageRef[] private msgsForwardedValue;
    uint32 private numMsgsWithForwardGas;
    
    // The address that initiated async calls
    address private initiator;
    // The number of nested async modifier runs
    int private asyncModifierNum;
    
    // Store map of refunds that were failed to send
    mapping(address => uint256) private pendingRefund;

    // Events
    event MessageEnqueued(uint160 indexed messageId, address indexed from, address indexed to, uint256 value);
    event MessageReady(uint160 indexed messageId);
    event MessageDelivered(uint160 indexed messageId, bool success);
    event ResponseFailed(
        address indexed from,
        address indexed to,
        bool success,
        bytes response,
        uint256 requestId,
        uint256 responseFeeCredit
    );
    event CallFailed(
        address indexed from,
        address indexed to,
        uint256 value,
        Nil.Token[] tokens,
        bytes callData
    );
    event RefundClaimed(address indexed recipient, uint256 amount);

    // Helper function to create a deep copy of tokens array
    function _copyTokens(Nil.Token[] memory tokens) internal pure returns (Nil.Token[] memory) {
        Nil.Token[] memory tokensCopy = new Nil.Token[](tokens.length);
        for (uint i = 0; i < tokens.length; i++) {
            tokensCopy[i] = tokens[i];
        }
        return tokensCopy;
    }

    // Queue management functions
    function enqueueMessage(
        MessageParams memory params,
        Nil.Token[] memory tokens,
        bytes memory data
    ) internal returns (MessageRef memory) {
        uint32 shardId = uint32(Nil.getShardId(params.to));
        uint64 messageId = outMsgCount[shardId]++;
        
        // Create a deep copy of tokens
        Nil.Token[] memory tokensCopy = _copyTokens(tokens);
        
        messages[shardId].push(Message({
            id: messageId,
            from: params.from,
            to: params.to,
            refundTo: params.refundTo,
            bounceTo: params.bounceTo,
            value: params.value,
            tokens: tokensCopy,
            forwardKind: params.forwardKind,
            feeCredit: params.feeCredit,
            data: data,
            requestId: params.requestId,
            responseFeeCredit: params.responseFeeCredit,
            isDeploy: params.isDeploy,
            salt: params.salt
        }));
        
        emit MessageEnqueued(messageId, params.from, params.to, params.value);
        
        return MessageRef({
            shardId: shardId,
            messageIndex: uint32(messages[shardId].length - 1)
        });
    }
    
    /**
     * @dev Gets pending messages for a specific shard
     */
    function getPendingMessages(uint32 shardId, uint64 fromId, uint32 count) external view returns (Message[] memory) {
        ////console.log("getPending messages: shardId=%_, fromId=%_, count=%_", shardId, fromId, count);
        require(count > 0, "Invalid count");
        require(fromId <= outMsgCount[shardId], "Invalid fromId");
        require(shardId < _N_SHARDS, "Invalid shardId");

        // Check if there are no messages to return
        if (fromId == outMsgCount[shardId] || messages[shardId].length == 0) {
            return new Message[](0);
        }
        
        uint64 toId = fromId + uint64(count);
        if (toId > outMsgCount[shardId]) {
            toId = outMsgCount[shardId];
        }
        
        uint32 resultCount = uint32(toId - fromId);
        Message[] memory result = new Message[](resultCount);
        
        uint64 fromIndex = fromId - messages[shardId][0].id;
        
        for (uint32 i = 0; i < resultCount; i++) {
            result[i] = messages[shardId][fromIndex + i];
        }
        
        return result;
    }

    /**
     * @dev Prunes processed messages
     */
    function pruneProcessedMessages(uint64[] memory inMsgIds) external returns (uint32) {
        uint32 prunedCount = 0;
        
        for (uint32 i = 0; i < _N_SHARDS; i++) {
            uint64 lastId = inMsgIds[i];
            require(lastId <= outMsgCount[i], "Invalid lastId");
            
            while (messages[i].length > 0 && messages[i][0].id <= lastId) {
                _removeOldestMessage(i);
                prunedCount++;
            }
        }
        
        return prunedCount;
    }
    
    // Helper to remove the oldest message from a shard queue
    function _removeOldestMessage(uint32 shardId) private {
        if (messages[shardId].length == 0) return;
        
        for (uint j = 0; j < messages[shardId].length - 1; j++) {
            messages[shardId][j] = messages[shardId][j + 1];
        }
        messages[shardId].pop();
    }

    /**
     * @dev Initialize Relayer for further cross-shard transactions. Should be always called before any async call.
     */
    function startAsync() public payable {
        require(initiator == address(0) || msg.sender == initiator, "startAsync: async send from different addresses");
        initiator = msg.sender;
        
        if (asyncModifierNum == 0) {
            ////console.log("Relayer start");
            numMsgsWithForwardGas = 0;
            delete msgsForwardedRemaining;
            delete msgsForwardedPercentage;
            delete msgsForwardedValue;
        }
        
        asyncModifierNum++;
    }

    /**
     * @dev Finalize all async calls by forwarding gas to all transactions.
     * @param gas The amount of gas available for forwarding.
     */
    function finalizeAsync(uint gas) public payable {
        asyncModifierNum--;
        require(asyncModifierNum >= 0, "finalizeAsync: wrong async modifier run");
        
        if (asyncModifierNum != 0) {
            return;
        }

        //console.log("Relayer finalize: gas=%_, num=%_", gas, numMsgsWithForwardGas);
        _forwardGas(gas);
        _resetForwarding();
    }

    function _forwardGas(uint gas) internal {
        uint feeCredit = gas * tx.gasprice;
        //console.log("_forwardGas: gas=%_, num=%_", gas, numMsgsWithForwardGas);

        if (numMsgsWithForwardGas == 0) {
            return;
        }

        // Process VALUE forwarded messages
        for (uint256 i = 0; i < msgsForwardedValue.length; i++) {
            MessageRef memory messageRef = msgsForwardedValue[i];
            Message storage message = messages[messageRef.shardId][messageRef.messageIndex];
        
            //console.log("_forwardGas value: to=%_, fee=%_", message.to, message.feeCredit);
            require(feeCredit >= message.feeCredit, "Not enough feeCredit for ForwardValue");
            feeCredit -= message.feeCredit;
        }

        // Process PERCENTAGE forwarded messages
        uint percentageTotal = 0;
        uint baseFeeCredit = feeCredit;
        
        for (uint256 i = 0; i < msgsForwardedPercentage.length; i++) {
            MessageRef memory messageRef = msgsForwardedPercentage[i];
            Message storage message = messages[messageRef.shardId][messageRef.messageIndex];
            
            require(message.forwardKind == Nil.FORWARD_PERCENTAGE, "Invalid percentage forwarding");

            percentageTotal += message.feeCredit;
            if (percentageTotal > 100) {
                revert("Total percentage is greater than 100");
            }

            message.feeCredit = (message.feeCredit * baseFeeCredit) / 100;

            if (feeCredit < message.feeCredit) {
                message.feeCredit = feeCredit;
                feeCredit = 0;
            } else {
                feeCredit -= message.feeCredit;
            }

            //console.log("_forwardGas percentage: fee=%_", message.feeCredit);
        }

        // Process REMAINING forwarded messages
        if (msgsForwardedRemaining.length != 0) {
            if (feeCredit == 0) {
                revert("Not enough feeCredit for ForwardRemaining");
            }
            uint feeCreditForward = feeCredit / msgsForwardedRemaining.length;
            feeCredit = 0;
            
            for (uint256 i = 0; i < msgsForwardedRemaining.length; i++) {
                MessageRef memory messageRef = msgsForwardedRemaining[i];
                Message storage message = messages[messageRef.shardId][messageRef.messageIndex];

                //console.log("_forwardGas remaining: to=%_, fee=%_", message.to, feeCreditForward);

                require(message.forwardKind == Nil.FORWARD_REMAINING);
                message.feeCredit = feeCreditForward;
            }
        }

        if (feeCredit != 0) {
            //console.log("_forwardGas: return fee %_ to %_", feeCredit, msg.sender);
            bytes memory data = abi.encodeWithSignature("nilReceive()");
            (bool success,) = payable(msg.sender).call{value: feeCredit}(data);
            if (!success) {
                revert("Failed to return feeCredit");
            }
        }
    }

    function _resetForwarding() internal {
        initiator = address(0);
        numMsgsWithForwardGas = 0;
        delete msgsForwardedRemaining;
        delete msgsForwardedPercentage;
        delete msgsForwardedValue;
    }

    /**
     * @dev Processes the forwarding kind of a message.
     */
    function processForwardKind(
        uint8 forwardKind,
        MessageRef memory messageRef
    ) internal {
        if (forwardKind == Nil.FORWARD_REMAINING) {
            msgsForwardedRemaining.push(messageRef);
            numMsgsWithForwardGas++;
            //console.log("processForwardKind FORWARD_REMAINING: num=%_", numMsgsWithForwardGas);
        } else if (forwardKind == Nil.FORWARD_PERCENTAGE) {
            msgsForwardedPercentage.push(messageRef);
            numMsgsWithForwardGas++;
            //console.log("processForwardKind FORWARD_PERCENTAGE: num=%_", numMsgsWithForwardGas);
        } else if (forwardKind == Nil.FORWARD_VALUE) {
            msgsForwardedValue.push(messageRef);
            numMsgsWithForwardGas++;
            //console.log("processForwardKind FORWARD_VALUE: num=%_", numMsgsWithForwardGas);
        } else if (forwardKind == Nil.FORWARD_NONE) {
            numMsgsWithForwardGas++;
        } else {
            revert("Invalid forwardKind");
        }
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

        require(asyncModifierNum > 0, "Relayer not initialized");

        // Process request ID and response gas
        uint256 responseFeeCredit = 0;
        if (requestId != 0) {
            require(responseGas > 0, "responseGas must be greater than 0");
            responseFeeCredit = responseGas * tx.gasprice;
            require(feeCredit >= responseFeeCredit, "feeCredit must be greater than responseFeeCredit");
            feeCredit -= responseFeeCredit;
        }

        // Set default addresses
        address actualRefundTo = refundTo == address(0) ? msg.sender : refundTo;
        address actualBounceTo = bounceTo == address(0) ? msg.sender : bounceTo;

        // Deduct tokens from sender
        NilTokenManager(Nil.getTokenManagerAddress()).deductForRelay(msg.sender, to, tokens);

        // Create parameters struct to reduce stack depth
        MessageParams memory params = MessageParams({
            from: msg.sender,
            to: Nil.getRelayerAddress(Nil.getShardId(to)),
            refundTo: actualRefundTo,
            bounceTo: actualBounceTo,
            value: value,
            forwardKind: forwardKind,
            feeCredit: feeCredit,
            requestId: requestId,
            responseFeeCredit: responseFeeCredit,
            isDeploy: false,
            salt: 0
        });

        // Prepare the receiveTx calldata
        bytes memory data = abi.encodeWithSelector(
            this.receiveTx.selector, 
            msg.sender, 
            to,
            outMsgCount[uint32(Nil.getShardId(to))],
            actualBounceTo, 
            value, 
            tokens, 
            callData, 
            requestId, 
            responseFeeCredit
        );

        // Enqueue the message
        MessageRef memory messageRef = enqueueMessage(params, tokens, data);
        processForwardKind(forwardKind, messageRef);

        //console.log("sendTx done: shardId=%_, messageId=%_", messageRef.shardId, messageRef.messageIndex);
    }

    /**
     * @dev Sends a deploy transaction.
     */
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
        //console.log("sendTxDeploy: to=%_, from=%_", to, msg.sender);

        require(asyncModifierNum > 0, "Relayer not initialized");

        // Set default addresses
        address actualRefundTo = refundTo == address(0) ? msg.sender : refundTo;
        address actualBounceTo = bounceTo == address(0) ? msg.sender : bounceTo;

        // Create parameters struct to reduce stack depth
        MessageParams memory params = MessageParams({
            from: msg.sender,
            to: Nil.getRelayerAddress(Nil.getShardId(to)),
            refundTo: actualRefundTo,
            bounceTo: actualBounceTo,
            value: value,
            forwardKind: forwardKind,
            feeCredit: feeCredit,
            requestId: 0,
            responseFeeCredit: 0,
            isDeploy: true,
            salt: salt
        });

        // Prepare the receiveTxDeploy calldata
        bytes memory data = abi.encodeWithSelector(
            this.receiveTxDeploy.selector,
            msg.sender,
            to,
            outMsgCount[uint32(Nil.getShardId(to))],
            actualBounceTo,
            value,
            salt,
            callData
        );

        // Enqueue the message with empty tokens array
        Nil.Token[] memory emptyTokens = new Nil.Token[](0);
        MessageRef memory messageRef = enqueueMessage(params, emptyTokens, data);
        processForwardKind(forwardKind, messageRef);
        
        //console.log("sendTxDeploy pushed: shardId=%_, messageId=%_", messageRef.shardId, messageRef.messageIndex);
    }

    /**
     * @dev Handles the receipt of a transaction.
     */
    function receiveTx(
        address from,
        address to,
        uint64 messageId,
        address bounceTo,
        uint value,
        Nil.Token[] memory tokens,
        bytes memory callData,
        uint256 requestId,
        uint256 responseFeeCredit
    ) public payable returns(bytes memory) {
        uint32 shardId = uint32(Nil.getShardId(from));
        require(inMsgCount[shardId]++ == messageId, "Invalid message ID");

        //console.log("receiveTx: gas=%_, to=%_, from=%_, messageId=%_", gasleft(), to, from, messageId);

        // Credit tokens to the recipient
        NilTokenManager(Nil.getTokenManagerAddress()).creditForRelay(to, tokens);

        // Execute the call
        uint gasForCall = calculateGasForTargetCall(requestId != 0);
        (bool success, bytes memory returnData) = to.call{value: value, gas: gasForCall}(callData);

        // Reset token context after the call
        NilTokenManager(Nil.getTokenManagerAddress()).resetTxTokens();

        //console.log("receiveTx: success=%_, gasleft=%_", success, gasleft());

        // Handle response or bounce based on outcome
        if (requestId != 0) {
            return _processResponseMessage(from, to, success, value, returnData, requestId, responseFeeCredit);
        } else if (!success) {
            return _processBounceMessage(from, to, bounceTo, value, tokens, callData, returnData);
        }

        return returnData;
    }

    /**
     * @dev Calculates the gas required for a target call.
     * @param request Indicates whether the call is a request.
     * @return The amount of gas required for the call of the target contract.
     */
    function calculateGasForTargetCall(bool request) internal view returns(uint) {
        uint gasForResponse = 50_000;
        uint requiredGasForFinish = 50_000;

        if (request) {
            requiredGasForFinish += gasForResponse;
        }

        if (gasleft() < requiredGasForFinish) {
            requiredGasForFinish = 0;
        } else {
            requiredGasForFinish = gasleft() - requiredGasForFinish;
        }
        return requiredGasForFinish;
    }

    /**
     * @dev Process a message that requires a response
     */
    function _processResponseMessage(
        address from,
        address to,
        bool success,
        uint256 value,
        bytes memory returnData,
        uint256 requestId,
        uint256 responseFeeCredit
    ) internal returns (bytes memory) {
        printRevertData("receiveTx request", returnData);
        uint256 returnValue = success ? 0 : value;
        
        // Create response parameters
        MessageParams memory params = MessageParams({
            from: to,
            to: Nil.getRelayerAddress(Nil.getShardId(from)),
            refundTo: from,
            bounceTo: from,
            value: returnValue,
            forwardKind: Nil.FORWARD_REMAINING,
            feeCredit: 0,
            requestId: 0,
            responseFeeCredit: 0,
            isDeploy: false,
            salt: 0
        });

        // Prepare response data
        bytes memory data = abi.encodeWithSelector(
            this.receiveTxResponse.selector, 
            to, 
            from, 
            outMsgCount[uint32(Nil.getShardId(from))],
            returnValue, 
            success, 
            returnData, 
            requestId, 
            responseFeeCredit
        );
        
        // Enqueue response message
        Nil.Token[] memory emptyTokens = new Nil.Token[](0);
        enqueueMessage(params, emptyTokens, data);
        
        return bytes("");
    }

    /**
     * @dev Process a bounce message for failed transactions
     */
    function _processBounceMessage(
        address from,
        address to,
        address bounceTo,
        uint256 value,
        Nil.Token[] memory tokens,
        bytes memory callData,
        bytes memory returnData
    ) internal returns (bytes memory) {
        printRevertData("receiveTx call failed", returnData);

        emit CallFailed(from, to, value, tokens, callData);

        // Deduct tokens from recipient for bounce
        NilTokenManager(Nil.getTokenManagerAddress()).deductForRelay(to, address(this), tokens);
        
        // Create bounce parameters
        MessageParams memory params = MessageParams({
            from: to,
            to: Nil.getRelayerAddress(Nil.getShardId(from)),
            refundTo: address(this),
            bounceTo: address(this),
            value: value,
            forwardKind: Nil.FORWARD_REMAINING,
            feeCredit: 100_000 * tx.gasprice,
            requestId: 0,
            responseFeeCredit: 0,
            isDeploy: false,
            salt: 0
        });
        
        // Prepare bounce data
        bytes memory data = abi.encodeWithSelector(
            this.receiveTxBounce.selector, 
            to,
            bounceTo, 
            outMsgCount[uint32(Nil.getShardId(bounceTo))],
            value, 
            tokens, 
            returnData
        );
        
        // Enqueue bounce message
        enqueueMessage(params, tokens, data);
        
        return bytes("");
    }

    /**
     * @dev Handles the deployment of a contract.
     */
    function receiveTxDeploy(
        address from,
        address to,
        uint64 messageId,
        address bounceTo,
        uint256 value,
        uint256 salt,
        bytes memory code
    ) public payable returns(bytes memory) {
        uint32 shardId = uint32(Nil.getShardId(from));
        require(inMsgCount[shardId]++ == messageId, "Invalid message ID");

        //console.log("receiveTxDeploy: gas=%_, to=%_, salt=%_, messageId=%_", gasleft(), to, salt, messageId);

        // Deploy the contract using CREATE2
        address addr;
        assembly {
            addr := create2(value, add(code, 0x20), mload(code), salt)
        }
        bool success = addr != address(0);

        //console.log("receiveTxDeploy: addr=%_", addr);

        if (!success) {
            return _processDeployBounce(from, bounceTo, value);
        }

        return bytes("");
    }

    /**
     * @dev Process a bounce message for failed deployments
     */
    function _processDeployBounce(
        address from,
        address bounceTo,
        uint256 value
    ) internal returns (bytes memory) {
        printRevertData("receiveTxDeploy call failed", "");

        // Create bounce parameters
        MessageParams memory params = MessageParams({
            from: address(this),
            to: Nil.getRelayerAddress(Nil.getShardId(from)),
            refundTo: address(this),
            bounceTo: address(this),
            value: value,
            forwardKind: Nil.FORWARD_REMAINING,
            feeCredit: 100_000 * tx.gasprice,
            requestId: 0,
            responseFeeCredit: 0,
            isDeploy: false,
            salt: 0
        });
        
        // Prepare bounce data
        bytes memory data = abi.encodeWithSelector(
            this.receiveTxBounce.selector, 
            address(this),
            bounceTo, 
            outMsgCount[uint32(Nil.getShardId(bounceTo))],
            value, 
            new Nil.Token[](0), 
            bytes("")
        );
        
        // Enqueue bounce message
        Nil.Token[] memory emptyTokens = new Nil.Token[](0);
        enqueueMessage(params, emptyTokens, data);
        
        return bytes("");
    }

    /**
     * @dev Handles the response of a transaction.
     */
    function receiveTxResponse(
        address from,
        address to,
        uint64 messageId,
        uint256 value,
        bool success,
        bytes memory response,
        uint256 requestId,
        uint256 responseFeeCredit
    ) public payable {
        uint32 shardId = uint32(Nil.getShardId(from));
        require(inMsgCount[shardId]++ == messageId, "Invalid message ID");
        
        // Calculate gas from responseFeeCredit using current gas price
        uint gas = responseFeeCredit / tx.gasprice;
        
        // Prepare onFallback calldata
        bytes memory data = abi.encodeWithSignature(
            "onFallback(uint256,bool,bytes)", 
            requestId, 
            success, 
            response
        );
        
        // Call the target with the response
        (bool s, ) = to.call{gas: gas, value: value}(data);
        if (!s) {
            emit ResponseFailed(to, from, success, response, requestId, responseFeeCredit);
        }
    }

    /**
     * @dev Handles the bounce of a failed transaction.
     */
    function receiveTxBounce(
        address from,
        address to,
        uint64 messageId,
        uint value,
        Nil.Token[] memory tokens,
        bytes memory callData
    ) public payable {
        uint32 shardId = uint32(Nil.getShardId(from));
        require(inMsgCount[shardId]++ == messageId, "Invalid message ID");
        
        printRevertData("Bounce tx", callData);
        //console.log("bounce: value=%_, to=%_", value, to);
        
        // Credit tokens back to the original sender
        NilTokenManager(Nil.getTokenManagerAddress()).creditForRelay(to, tokens);
        
        // Reset token context
        NilTokenManager(Nil.getTokenManagerAddress()).resetTxTokens();

        // Call the bounce method on the target
        bytes memory data = abi.encodeWithSignature("bounce(bytes)", callData);
        (bool success, bytes memory returnData) = to.call{value: value}(data);
        if (!success) {
            // Save the value for the future refund
            pendingRefund[to] += value;
            printRevertData("Bounce call failed", returnData);
        }
    }

    /**
     * @dev Utility function to print revert data in a readable format.
     */
    function printRevertData(string memory str, bytes memory returnData) internal pure {
        if (returnData.length > 68) {
            assembly {
                returnData := add(returnData, 0x04)
            }
            string memory reason = abi.decode(returnData, (string));
            //console.log("%_: %_", str, reason);
        } else {
            //console.log("%_: <no revert reason>", str);
        }
    }

    /**
     * @dev Allows users to claim their pending refunds
     */
    function claimPendingRefund() external returns (uint256) {
        uint256 amount = pendingRefund[msg.sender];
        require(amount > 0, "No pending refund");
        
        // Reset refund amount before transfer to prevent reentrancy
        pendingRefund[msg.sender] = 0;
        
        bytes memory data = abi.encodeWithSignature("nilReceive()");
        (bool success,) = payable(msg.sender).call{value: amount}(data);

        // If transfer fails, restore the pending refund
        if (!success) {
            pendingRefund[msg.sender] = amount;
            revert("Failed to send refund");
        }
        
        emit RefundClaimed(msg.sender, amount);
        return amount;
    }

    /**
     * @dev Returns the pending refund amount for an address
     */
    function getPendingRefund(address account) external view returns (uint256) {
        return pendingRefund[account];
    }

    function castToArray(uint64[_N_SHARDS] memory arr) internal pure returns (uint64[] memory) {
        uint64[] memory result = new uint64[](arr.length);
        for (uint32 i = 0; i < arr.length; i++) {
            result[i] = uint64(arr[i]);
        }
        return result;
    }

    /**
     * @dev Returns the current count of incoming messages for each shard
     */
    function getInMsgCount() public view returns (uint64[] memory) {
        return castToArray(inMsgCount);
    }

    /**
     * @dev Returns the current count of outgoing messages for each shard
     */
    function getOutMsgCount() public view returns (uint64[] memory) {
        return castToArray(outMsgCount);
    }

    /**
     * @dev Returns the current block number for each shard
     */
    function getCurrentBlockNumber() public view returns (uint64[] memory) {
        return castToArray(currentBlockNumber);
    }

    /**
     * @dev Updates the current block number for each shard
     */
    function updateCurrentBlockNumber(uint64[_N_SHARDS] memory blockNumbers) public {
        for (uint32 i = 0; i < _N_SHARDS; i++) {
            currentBlockNumber[i] = blockNumbers[i];
        }
    }

    /**
     * @dev Returns the message queue length for a specific shard
     */
    function getMessageQueueLength(uint32 shardId) public view returns (uint256) {
        return messages[shardId].length;
    }

    /**
     * @dev Returns the current state of the forwarding
     */
    function getForwardingState() public view returns (
        bool initialized,
        uint32 messageCount,
        uint256 remainingCount,
        uint256 percentageCount,
        uint256 valueCount
    ) {
        return (
            asyncModifierNum > 0,
            numMsgsWithForwardGas,
            msgsForwardedRemaining.length,
            msgsForwardedPercentage.length,
            msgsForwardedValue.length
        );
    }

    /**
     * @dev Returns the current async state
     */
    function getAsyncState() public view returns (
        int asyncNum,
        address asyncInitiator
    ) {
        return (
            asyncModifierNum,
            initiator
        );
    }
}

