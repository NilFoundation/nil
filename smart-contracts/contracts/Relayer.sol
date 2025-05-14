// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./NilTokenManager.sol";
import "../system/console.sol";
import "./IterableMapping.sol";

/**
 * @title Relayer
 * @dev This contract facilitates relaying transactions, handling responses, and managing token credits.
 * It now contains a message queue for asynchronous cross-shard communication.
 */
contract Relayer {
    using IterableMapping for IterableMapping.Map;

    // Message status enum
    enum MessageStatus {
        Pending,    // Initial state - waiting for processing
        Ready,      // Ready to be processed (gas allocated)
        Delivered,  // Successfully delivered and processed
        Failed      // Failed to process
    }

    // Structure for cross-shard messages
    struct Message {
        uint160 id;                  // Unique message identifier
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
        MessageStatus status;        // Message status
    }
    
    // Message queue data
    uint160 private nextMessageId = 1;
    mapping(uint160 => Message) private messages;
    IterableMapping.Map private pendingMessageIds;   // Messages waiting for gas allocation
    IterableMapping.Map private readyMessageIds;     // Messages ready to be processed
    IterableMapping.Map private processedMessageIds; // Delivered or failed messages
    
    // For async call management
    uint8[] private msgsForwardedRemaining;
    uint8[] private msgsForwardedPercentage;
    uint8[] private msgsForwardedValue;
    uint32 private numMsgsWithForwardGas;
    bool private forwardingInitialized;
    mapping(address => uint256) private pendingRefund;

    // Events

    /**
     * @dev Emitted when a message is enqueued for processing.
     * @param messageId The ID of the message.
     * @param from The address of the sender.
     * @param to The address of the recipient.
     * @param value The value transferred with the message.
     */
    event MessageEnqueued(
        uint160 indexed messageId, 
        address indexed from, 
        address indexed to, 
        uint256 value
    );

    /**
     * @dev Emitted when a message is ready to be processed.
     * @param messageId The ID of the message that is ready.
     */
    event MessageReady(uint160 indexed messageId);
    
    /**
     * @dev Emitted when a message has been successfully delivered.
     * @param messageId The ID of the delivered message.
     * @param success Whether the message execution was successful.
     */
    event MessageDelivered(uint160 indexed messageId, bool success);
    
    /**
     * @dev Emitted when a response fails to execute.
     * @param from The address that initiated the call.
     * @param to The target address of the call.
     * @param success Whether the response was successful.
     * @param response The response data.
     * @param requestId The ID of the request.
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
     * @dev Emitted when a refund is claimed.
     * @param recipient The address that received the refund.
     * @param amount The amount of Ether refunded.
     */
    event RefundClaimed(address indexed recipient, uint256 amount);

    // Queue management functions
    
    /**
     * @dev Enqueues a message for asynchronous processing
     * @param from Source address
     * @param to Destination address
     * @param refundTo Address to refund in case of failure
     * @param bounceTo Address to bounce to in case of failure
     * @param value Value to send
     * @param tokens Tokens to send
     * @param forwardKind Type of gas forwarding
     * @param feeCredit Fee credit to use
     * @param data Call data
     * @param requestId Request ID for responses
     * @param responseFeeCredit Fee credit allocated for response
     * @param isDeploy Whether this is a deploy message
     * @param salt Salt for deploy messages
     * @return id The ID of the enqueued message
     */
    function enqueueMessage(
        address from,
        address to,
        address refundTo,
        address bounceTo,
        uint256 value,
        Nil.Token[] memory tokens,
        uint8 forwardKind,
        uint256 feeCredit,
        bytes memory data,
        uint256 requestId,
        uint256 responseFeeCredit,
        bool isDeploy,
        uint256 salt
    ) public returns (uint160) {
        uint160 messageId = nextMessageId++;
        
        // Create a deep copy of tokens
        Nil.Token[] memory tokensCopy = new Nil.Token[](tokens.length);
        for (uint i = 0; i < tokens.length; i++) {
            tokensCopy[i] = tokens[i];
        }
        
        messages[messageId] = Message({
            id: messageId,
            from: from,
            to: to,
            refundTo: refundTo,
            bounceTo: bounceTo,
            value: value,
            tokens: tokensCopy,
            forwardKind: forwardKind,
            feeCredit: feeCredit,
            data: data,
            requestId: requestId,
            responseFeeCredit: responseFeeCredit,
            isDeploy: isDeploy,
            salt: salt,
            status: MessageStatus.Pending
        });
        
        pendingMessageIds.set(address(uint160(messageId)), 1);
        
        emit MessageEnqueued(messageId, from, to, value);
        
        return messageId;
    }
    
    /**
     * @dev Marks a message as ready for processing (with gas allocation)
     * @param messageId ID of the message
     */
    function markMessageReady(uint160 messageId) internal {
        require(messages[messageId].id == messageId, "Message doesn't exist");
        require(messages[messageId].status == MessageStatus.Pending, "Message not pending");
        
        messages[messageId].status = MessageStatus.Ready;
        pendingMessageIds.remove(address(uint160(messageId)));
        readyMessageIds.set(address(uint160(messageId)), 1);
        
        emit MessageReady(messageId);
    }
    
    /**
     * @dev Marks a message as delivered (called by validators after processing)
     * @param messageId ID of the message
     * @param success
     * It must be called by the validator when block with this message is finalized 
     */
    function markMessageDelivered(uint160 messageId, bool success) external {
        require(messages[messageId].id == messageId, "Message doesn't exist");
        require(messages[messageId].status == MessageStatus.Ready, "Message not ready");
        
        messages[messageId].status = success ? MessageStatus.Delivered : MessageStatus.Failed;
        readyMessageIds.remove(address(uint160(messageId)));
        processedMessageIds.set(address(uint160(messageId)), 1);
        
        emit MessageDelivered(messageId, success);
    }
    
    /**
     * @dev Gets pending messages (waiting for gas allocation)
     * @param count Maximum number of messages to return
     * @return messageIds Array of message IDs
     */
    function getPendingMessages(uint256 count) external view returns (uint160[] memory) {
        return _getMessagesFromMap(pendingMessageIds, count);
    }
    
    /**
     * @dev Gets ready messages (with allocated gas, ready for processing)
     * @param count Maximum number of messages to return
     * @return messageIds Array of message IDs
     * It must be called by the validator when it is assembling a block.
     */
    function getReadyMessages(uint256 count) external view returns (uint160[] memory) {
        return _getMessagesFromMap(readyMessageIds, count);
    }
    
    /**
     * @dev Helper function to get messages from a map
     */
    function _getMessagesFromMap(IterableMapping.Map storage map, uint256 count) private view returns (uint160[] memory) {
        uint256 mapSize = map.size();
        uint256 resultCount = mapSize < count ? mapSize : count;
        
        uint160[] memory result = new uint160[](resultCount);
        
        for (uint256 i = 0; i < resultCount; i++) {
            address messageIdAddress = map.getKeyAtIndex(i);
            result[i] = uint160(messageIdAddress);
        }
        
        return result;
    }
    
    /**
     * @dev Gets a specific message by ID
     * @param messageId ID of the message
     * @return The message details
     */
    function getMessage(uint160 messageId) external view returns (
        address from,
        address to,
        address refundTo,
        address bounceTo,
        uint256 value,
        Nil.Token[] memory tokens,
        uint8 forwardKind,
        uint256 feeCredit,
        bytes memory data,
        uint256 requestId,
        uint256 responseFeeCredit,
        bool isDeploy,
        uint256 salt,
        MessageStatus status
    ) {
        Message storage message = messages[messageId];
        require(message.id == messageId, "Message doesn't exist");
        
        return (
            message.from,
            message.to,
            message.refundTo,
            message.bounceTo,
            message.value,
            message.tokens,
            message.forwardKind,
            message.feeCredit,
            message.data,
            message.requestId,
            message.responseFeeCredit,
            message.isDeploy,
            message.salt,
            message.status
        );
    }
    
    /**
     * @dev Prunes processed messages (delivered or failed)
     * @param count Maximum number of messages to prune
     * @return Number of messages pruned
     * It must be called by validator periodically(for example, once per block)
     */
    function pruneProcessedMessages(uint256 count) external returns (uint256) {
        uint256 processedCount = processedMessageIds.size();
        uint256 prunedCount = 0;
        
        for (uint256 i = 0; i < processedCount && i < count; i++) {
            address messageIdAddress = processedMessageIds.getKeyAtIndex(0); // Always remove first
            uint160 messageId = uint160(messageIdAddress);
            
            // Delete the message data
            delete messages[messageId];
            // Remove from the processed list
            processedMessageIds.remove(messageIdAddress);
            prunedCount++;
        }
        
        return prunedCount;
    }

    /**
     * @dev Execute a ready message - used by validators to process messages
     * @param messageId ID of the message to process
     * @return success Whether processing was successful
     * @return returnData Response data from processing
     * This function must be called by the validator to process the message
     */
    function executeMessage(uint160 messageId) external returns (bool success, bytes memory returnData) {
        Message storage message = messages[messageId];
        require(message.id == messageId, "Message doesn't exist");
        require(message.status == MessageStatus.Ready, "Message not ready for execution");
        
        if (message.isDeploy) {
            // Handle deploy message
            return _processDeploy(message);
        } else {
            // Handle regular execution message
            return _processCall(message);
        }
    }
    
    /**
     * @dev Process a deploy message
     */
    function _processDeploy(Message storage message) internal returns (bool success, bytes memory returnData) {
        address addr;
        
        // Credit any tokens to the new contract address
        if (message.tokens.length > 0) {
            NilTokenManager(Nil.getTokenManagerAddress()).creditForRelay(message.to, message.tokens);
        }
        
        // Deploy the contract using CREATE2
        bytes memory code = message.data;
        assembly {
            addr := create2(
                mload(add(message.slot, 0x60)), // message.value
                add(code, 0x20),
                mload(code),
                mload(add(message.slot, 0x1A0)) // message.salt
            )
        }
        
        success = addr != address(0);
        
        // Reset tokens after deployment attempt
        if (message.tokens.length > 0) {
            NilTokenManager(Nil.getTokenManagerAddress()).resetTxTokens();
        }
        
        if (!success) {
            // Handle failed deployment - prepare bounce message
            bytes memory bounceData = abi.encodeWithSelector(
                this.receiveTxBounce.selector, 
                message.bounceTo, 
                message.value, 
                message.tokens, 
                bytes("")
            );
            
            // Enqueue bounce message
            enqueueMessage(
                address(this),
                Nil.getRelayerAddress(Nil.getShardId(message.from)),
                message.refundTo,
                message.bounceTo,
                message.value,
                message.tokens,
                Nil.FORWARD_REMAINING,
                100_000 * tx.gasprice, // Standard fee for bounce
                bounceData,
                0,
                0,
                false,
                0
            );
            
            returnData = bytes("Deployment failed");
        } else {
            returnData = abi.encodePacked(addr);
        }
        
        return (success, returnData);
    }
    
    /**
     * @dev Process a regular call message
     */
    function _processCall(Message storage message) internal returns (bool success, bytes memory returnData) {
        // Credit tokens to destination before call
        if (message.tokens.length > 0) {
            NilTokenManager(Nil.getTokenManagerAddress()).creditForRelay(message.to, message.tokens);
        }
        
        // Execute the call
        (success, returnData) = message.to.call{value: message.value}(message.data);
        
        // Reset tokens after call
        if (message.tokens.length > 0) {
            NilTokenManager(Nil.getTokenManagerAddress()).resetTxTokens();
        }
        
        if (message.requestId != 0) {
            // This is a request with expected response
            uint256 returnValue = success ? 0 : message.value;
            
            // Prepare response data
            bytes memory responseData = abi.encodeWithSelector(
                this.receiveTxResponse.selector, 
                message.to, 
                message.from, 
                returnValue, 
                success, 
                returnData, 
                message.requestId, 
                message.responseFeeCredit
            );
            
            // Enqueue response message
            enqueueMessage(
                message.to,
                Nil.getRelayerAddress(Nil.getShardId(message.from)),
                message.refundTo,
                message.bounceTo,
                returnValue,
                new Nil.Token[](0), // No tokens in response
                Nil.FORWARD_REMAINING,
                0, // No fee credit needed for response handling
                responseData,
                0, // No request ID for response
                0, // No response fee credit for response
                false,
                0
            );
            
            return (success, returnData);
        }
        
        if (!success) {
            // Failed call, emit event
            emit CallFailed(message.from, message.to, message.value, message.tokens, message.data);
            
            // Deduct tokens from destination before bounce
            if (message.tokens.length > 0) {
                NilTokenManager(Nil.getTokenManagerAddress()).deductForRelay(message.to, address(this), message.tokens);
            }
            
            // Prepare bounce data
            bytes memory bounceData = abi.encodeWithSelector(
                this.receiveTxBounce.selector, 
                message.bounceTo, 
                message.value, 
                message.tokens, 
                returnData
            );
            
            // Enqueue bounce message
            enqueueMessage(
                address(this),
                Nil.getRelayerAddress(Nil.getShardId(message.from)),
                message.refundTo,
                message.bounceTo,
                message.value,
                message.tokens,
                Nil.FORWARD_REMAINING,
                100_000 * tx.gasprice, // Standard fee for bounce
                bounceData,
                0,
                0,
                false,
                0
            );
        }
        
        return (success, returnData);
    }

    function startAsync() public payable {
        require(!forwardingInitialized, "Relayer already initialized");
        console.log("Relayer start");
        numMsgsWithForwardGas = 0;
        delete msgsForwardedRemaining;
        delete msgsForwardedPercentage;
        delete msgsForwardedValue;
        forwardingInitialized = true;
    }

    function finalizeAsync(uint gas) public payable {
        console.log("Relayer finalize: gas=%_, num=%_", gas, numMsgsWithForwardGas);
        _forwardGas(gas);
        _resetForwarding();
    }

    function _forwardGas(uint gas) internal {
        uint feeCredit = gas * tx.gasprice;
        console.log("_forwardGas: gas=%_, num=%_", gas, numMsgsWithForwardGas);

        if (numMsgsWithForwardGas == 0) {
            return;
        }

        // Process VALUE forwarded messages first
        for (uint256 i = 0; i < msgsForwardedValue.length; i++) {
            uint256 messageId = uint256(uint160(pendingMessageIds.getKeyAtIndex(msgsForwardedValue[i])));
            Message storage msg = messages[messageId];
            
            console.log("_forwardGas value: to=%_, fee=%_", msg.to, msg.feeCredit);
            require(feeCredit >= msg.feeCredit, "forwardGas: not enough feeCredit for ForwardValue");
            feeCredit -= msg.feeCredit;
            
            markMessageReady(messageId);
        }

        // Process PERCENTAGE forwarded messages
        uint percentageTotal = 0;
        uint baseFeeCredit = feeCredit;
        for (uint256 i = 0; i < msgsForwardedPercentage.length; i++) {
            uint256 messageId = uint256(uint160(pendingMessageIds.getKeyAtIndex(msgsForwardedPercentage[i])));
            Message storage msg = messages[messageId];
            
            require(msg.forwardKind == Nil.FORWARD_PERCENTAGE, "forwardGas: invalid percentage forwarding");

            percentageTotal += msg.feeCredit;
            if (percentageTotal > 100) {
                revert("forwardGas: total percentage is greater than 100");
            }

            msg.feeCredit = (msg.feeCredit * baseFeeCredit) / 100;

            if (feeCredit < msg.feeCredit) {
                msg.feeCredit = feeCredit;
                feeCredit = 0;
            } else {
                feeCredit -= msg.feeCredit;
            }

            console.log("_forwardGas percentage: fee=%_", msg.feeCredit);
            markMessageReady(messageId);
        }

        // Process REMAINING forwarded messages
        if (msgsForwardedRemaining.length != 0) {
            if (feeCredit == 0) {
                revert("forwardGas: not enough feeCredit for ForwardRemaining");
            }
            uint feeCreditForward = feeCredit / msgsForwardedRemaining.length;
            feeCredit = 0;
            
            for (uint256 i = 0; i < msgsForwardedRemaining.length; i++) {
                uint256 messageId = uint256(uint160(pendingMessageIds.getKeyAtIndex(msgsForwardedRemaining[i])));
                Message storage msg = messages[messageId];

                console.log("_forwardGas remaining: to=%_, fee=%_", msg.to, feeCreditForward);

                require(msg.forwardKind == Nil.FORWARD_REMAINING);
                msg.feeCredit = feeCreditForward;
                markMessageReady(messageId);
            }
        }

        if (feeCredit != 0) {
            console.log("_forwardGas: return fee %_ to %_", feeCredit, msg.sender);
            bytes memory data = abi.encodeWithSignature("nilReceive()");
            (bool success,) = payable(msg.sender).call{value: feeCredit}(data);
            if (!success) {
                revert("forwardGas: failed to return feeCredit(probably nilReceive is not implemented)");
            }
        }
    }

    function _resetForwarding() internal {
        numMsgsWithForwardGas = 0;
        delete msgsForwardedRemaining;
        delete msgsForwardedPercentage;
        delete msgsForwardedValue;
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
        uint256 feeCredit,
        uint8 forwardKind,
        uint256 value,
        Nil.Token[] memory tokens,
        bytes memory callData,
        uint256 requestId,
        uint256 responseGas
    ) public payable {
        console.log("sendTx: to=%_, from=%_", to, msg.sender);

        require(forwardingInitialized, "Relayer not initialized");

        if (forwardKind == Nil.FORWARD_REMAINING) {
            msgsForwardedRemaining.push(uint8(pendingMessageIds.size()));
            numMsgsWithForwardGas++;
            console.log("sendTx FORWARD_REMAINING: num=%_", numMsgsWithForwardGas);
        } else if (forwardKind == Nil.FORWARD_PERCENTAGE) {
            msgsForwardedPercentage.push(uint8(pendingMessageIds.size()));
            numMsgsWithForwardGas++;
            console.log("sendTx FORWARD_PERCENTAGE: num=%_", numMsgsWithForwardGas);
        } else if (forwardKind == Nil.FORWARD_VALUE) {
            msgsForwardedValue.push(uint8(pendingMessageIds.size()));
            numMsgsWithForwardGas++;
            console.log("sendTx FORWARD_VALUE: num=%_", numMsgsWithForwardGas);
        } else if (forwardKind == Nil.FORWARD_NONE) {
            numMsgsWithForwardGas++;
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

        // Deduct tokens from sender
        NilTokenManager(Nil.getTokenManagerAddress()).deductForRelay(msg.sender, to, tokens);

        // Prepare the receiveTx calldata
        bytes memory data = abi.encodeWithSelector(
            this.receiveTx.selector, 
            msg.sender, 
            to, 
            bounceTo, 
            value, 
            tokens, 
            callData, 
            requestId, 
            responseFeeCredit
        );

        // Enqueue the message
        uint256 messageId = enqueueMessage(
            msg.sender,
            Nil.getRelayerAddress(Nil.getShardId(to)),
            refundTo,
            bounceTo,
            value,
            tokens,
            forwardKind,
            feeCredit,
            data,
            requestId,
            responseFeeCredit,
            false,
            0
        );
        
        console.log("sendTx done: messageId=%_", messageId);
    }

    /**
     * @dev Sends a deploy transaction.
     * @param to The target address (pre-computed contract address).
     * @param refundTo The address to refund in case of failure.
     * @param bounceTo The address to bounce the transaction to in case of failure.
     * @param feeCredit The fee credit for the transaction.
     * @param forwardKind The forwarding type.
     * @param value The amount of Ether to send.
     * @param salt The salt for the contract address creation.
     * @param callData The contract code.
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
        console.log("sendTxDeploy: to=%_, from=%_", to, msg.sender);

        require(forwardingInitialized, "Relayer not initialized");

        if (forwardKind == Nil.FORWARD_REMAINING) {
            msgsForwardedRemaining.push(uint8(pendingMessageIds.size()));
            numMsgsWithForwardGas++;
            console.log("sendTxDeploy FORWARD_REMAINING: num=%_", numMsgsWithForwardGas);
        } else if (forwardKind == Nil.FORWARD_PERCENTAGE) {
            msgsForwardedPercentage.push(uint8(pendingMessageIds.size()));
            numMsgsWithForwardGas++;
            console.log("sendTxDeploy FORWARD_PERCENTAGE: num=%_", numMsgsWithForwardGas);
        } else if (forwardKind == Nil.FORWARD_VALUE) {
            msgsForwardedValue.push(uint8(pendingMessageIds.size()));
            numMsgsWithForwardGas++;
            console.log("sendTxDeploy FORWARD_VALUE: num=%_", numMsgsWithForwardGas);
        } else if (forwardKind == Nil.FORWARD_NONE) {
            numMsgsWithForwardGas++;
        } else {
            revert("sendTxDeploy: invalid forwardKind");
        }

        if (refundTo == address(0)) {
            refundTo = msg.sender;
        }
        if (bounceTo == address(0)) {
            bounceTo = msg.sender;
        }

        // Prepare the receiveTxDeploy calldata
        bytes memory data = abi.encodeWithSelector(
            this.receiveTxDeploy.selector,
            msg.sender,
            to,
            bounceTo,
            value,
            salt,
            callData
        );

        // Enqueue the message
        uint256 messageId = enqueueMessage(
            msg.sender,
            Nil.getRelayerAddress(Nil.getShardId(to)),
            refundTo,
            bounceTo,
            value,
            new Nil.Token[](0),
            forwardKind,
            feeCredit,
            data,
            0,              // No request ID for deploy
            0,              // No response fee credit for deploy
            true,           // Is deploy
            salt
        );
        
        console.log("sendTxDeploy pushed: messageId=%_", messageId);
    }

    /**
     * @dev Handles the receipt of a transaction.
     * @param from The address that initiated the transaction.
     * @param to The target address of the transaction.
     * @param bounceTo The address to bounce to in case of failure.
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

        // Credit tokens to the recipient
        NilTokenManager(Nil.getTokenManagerAddress()).creditForRelay(to, tokens);

        // Execute the call
        (bool success, bytes memory returnData) = to.call{value: value}(callData);

        // Reset token context after the call
        NilTokenManager(Nil.getTokenManagerAddress()).resetTxTokens();

        console.log("receiveTx: success=%_, gasleft=%_", success, gasleft());

        if (requestId != 0) {
                        // This is a transaction that requires a response
            printRevertData("receiveTx request", returnData);
            uint256 returnValue = 0;
            if (!success) {
                returnValue = value;
            }
            
            // Prepare response data
            bytes memory data = abi.encodeWithSelector(
                this.receiveTxResponse.selector, 
                to, 
                from, 
                returnValue, 
                success, 
                returnData, 
                requestId, 
                responseFeeCredit
            );
            
            // Enqueue response message
            enqueueMessage(
                to,
                Nil.getRelayerAddress(Nil.getShardId(from)),
                from,
                from,
                returnValue,
                new Nil.Token[](0),
                Nil.FORWARD_REMAINING,
                0,
                data,
                0,
                0,
                false,
                0
            );
            
            return bytes("");
        }

        if (!success) {
            printRevertData("receiveTx call failed", returnData);

            emit CallFailed(from, to, value, tokens, callData);

            // Deduct tokens from recipient for bounce
            NilTokenManager(Nil.getTokenManagerAddress()).deductForRelay(to, address(this), tokens);
            
            // Prepare bounce data
            bytes memory data = abi.encodeWithSelector(
                this.receiveTxBounce.selector, 
                bounceTo, 
                value, 
                tokens, 
                returnData
            );
            
            // Enqueue bounce message
            enqueueMessage(
                address(this),
                Nil.getRelayerAddress(Nil.getShardId(from)),
                address(this),
                address(this),
                value,
                tokens,
                Nil.FORWARD_REMAINING,
                100_000 * tx.gasprice,
                data,
                0,
                0,
                false,
                0
            );
            
            return bytes("");
        }

        return returnData;
    }

    /**
     * @dev Handles the deployment of a contract.
     * @param from The address that initiated the deployment.
     * @param to The target address (pre-computed contract address).
     * @param bounceTo The address to bounce to in case of failure.
     * @param value The amount of Ether sent with the deployment.
     * @param salt The salt for the contract address creation.
     * @param code The contract code.
     * @return The return data from the deployment.
     */
    function receiveTxDeploy(
        address from,
        address to,
        address bounceTo,
        uint256 value,
        uint256 salt,
        bytes memory code
    ) public payable returns(bytes memory) {
        console.log("receiveTxDeploy: gas=%_, to=%_, salt=%_", gasleft(), to, salt);

        // Deploy the contract using CREATE2
        address addr;
        assembly {
            addr := create2(value, add(code, 0x20), mload(code), salt)
        }
        bool success = addr != address(0);

        console.log("receiveTxDeploy: addr=%_", addr);

        if (!success) {
            printRevertData("receiveTxDeploy call failed", "");

            // Prepare bounce data
            bytes memory data = abi.encodeWithSelector(
                this.receiveTxBounce.selector, 
                bounceTo, 
                value, 
                new Nil.Token[](0), 
                bytes("")
            );
            
            // Enqueue bounce message
            enqueueMessage(
                address(this),
                Nil.getRelayerAddress(Nil.getShardId(from)),
                address(this),
                address(this),
                value,
                new Nil.Token[](0),
                Nil.FORWARD_REMAINING,
                100_000 * tx.gasprice,
                data,
                0,
                0,
                false,
                0
            );
            
            return bytes("");
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
        // Calculate gas from responseFeeCredit using current gas price
        uint gas = responseFeeCredit / Nil.getGasPrice(address(this));
        
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
        console.log("bounce: value=%_, to=%_", value, to);
        
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
     * @param str A string to prefix the revert data.
     * @param returnData The return data containing revert information.
     */
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

/**
 * @dev Allows users to claim their pending refunds
 * @return The amount claimed
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
        revert("Failed to send refund via nilReceive()");
    }
    
    emit RefundClaimed(msg.sender, amount);
    return amount;
}

/**
 * @dev Returns the pending refund amount for an address
 * @param account The address to check
 * @return The pending refund amount
 */
function getPendingRefund(address account) external view returns (uint256) {
    return pendingRefund[account];
}