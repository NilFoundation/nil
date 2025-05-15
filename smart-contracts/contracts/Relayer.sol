// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./NilTokenManager.sol";
import "../../nil/contracts/solidity/system/console.sol";
// import "../system/console.sol";
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
        Ready,      // Ready to be processed (fee credit allocated)
        Delivered,  // Successfully delivered and processed
        Failed      // Failed to process
    }

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
    
    // Message queue data
    mapping(uint32 => Message[]) messages;
    mapping(uint32 => uint64) private inMsgCount;
    mapping(uint32 => uint64) private outMsgCount;

    struct MessageRef {
        uint32 shardId;
        uint32 messageIndex;
        uint64 messageId;
    }
    
    // For async call management
    MessageRef[] private msgsForwardedRemaining;
    MessageRef[] private msgsForwardedPercentage;
    MessageRef[] private msgsForwardedValue;
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
    ) public returns (MessageRef memory) {
        uint32 shardId = uint32(Nil.getShardId(to));
        uint64 messageId = outMsgCount[shardId]++;
        
        // Create a deep copy of tokens
        Nil.Token[] memory tokensCopy = new Nil.Token[](tokens.length);
        for (uint i = 0; i < tokens.length; i++) {
            tokensCopy[i] = tokens[i];
        }
        
        messages[shardId].push(Message({
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
            salt: salt
        }));
        
        emit MessageEnqueued(messageId, from, to, value);
        
        return MessageRef({
            shardId: shardId,
            messageIndex: uint32(messages[shardId].length - 1),
            messageId: messageId
        });
    }
    

    
    /**
     * @dev Gets pending messages for a specific shard
     * @param shardId The shard ID to get messages from
     * @param count The maximum number of messages to return
     * @return An array of pending messages
     */
    function getPendingMessages(uint32 shardId, uint32 count) external view returns (Message[] memory) {
        uint256 pendingCount = messages[shardId].length;
        uint256 resultCount = pendingCount < count ? pendingCount : count;
        
        Message[] memory result = new Message[](resultCount);
        
        for (uint256 i = 0; i < resultCount; i++) {
            result[i] = messages[shardId][i];
        }
        
        return result;
    }

    
    /**
    * @dev Gets a specific message by ID
    * @param messageId ID of the message
    * @return from The address that sent the message
    * @return to The destination address of the message
    * @return refundTo The address to refund in case of failure
    * @return bounceTo The address to bounce to in case of failure
    * @return value The value to transfer
    * @return tokens The tokens to transfer
    * @return forwardKind The forwarding kind
    * @return feeCredit The fee credit
    * @return data The call data
    * @return requestId The request ID for responses
    * @return responseFeeCredit The fee credit allocated for response
    * @return isDeploy Whether this is a deploy message
    * @return salt The salt for deploy messages
    * @return status The message status
    */
    // function getMessage(uint160 messageId) external view returns (
    //     address from,
    //     address to,
    //     address refundTo,
    //     address bounceTo,
    //     uint256 value,
    //     Nil.Token[] memory tokens,
    //     uint8 forwardKind,
    //     uint256 feeCredit,
    //     bytes memory data,
    //     uint256 requestId,
    //     uint256 responseFeeCredit,
    //     bool isDeploy,
    //     uint256 salt,
    //     MessageStatus status
    // ) {
    //     Message storage message = messages[messageId];
    //     require(message.id == messageId, "Message doesn't exist");
        
    //     return (
    //         message.from,
    //         message.to,
    //         message.refundTo,
    //         message.bounceTo,
    //         message.value,
    //         message.tokens,
    //         message.forwardKind,
    //         message.feeCredit,
    //         message.data,
    //         message.requestId,
    //         message.responseFeeCredit,
    //         message.isDeploy,
    //         message.salt,
    //         message.status
    //     );
    // }
    
    /**
     * @dev Prunes processed messages (delivered or failed)
     * @param inMsgIds is an array that stores for every shard the last message ID that has been processed
     * @return Number of messages pruned
     * It must be called by validator periodically(for example, once per block)
     */
    function pruneProcessedMessages(uint64[] memory inMsgIds) external returns (uint32) {
        uint32 prunedCount = 0;
        for (uint32 i = 0; i < Nil.SHARDS_NUM; i++) {
            uint64 lastId = inMsgIds[i];
            require(lastId <= outMsgCount[i], "pruneProcessedMessages: invalid lastId");
            while (messages[i].length > 0 && messages[i][0].id <= lastId) {
                messages[i].pop();
                prunedCount++;
            }
        }
        return prunedCount;
    }

    /**
     * @dev Executes a message that is ready for processing
     * @param message The message to execute
     * @return success Whether the execution was successful
     * @return returnData The data returned from the execution
     */
    // function executeMessage(Message memory message) external returns (bool success, bytes memory returnData) {
    //     if (message.isDeploy) {
    //         // Handle deploy message
    //         return _processDeploy(message);
    //     } else {
    //         // Handle regular execution message
    //         return _processCall(message);
    //     }
    // }
    
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
            MessageRef memory messageRef = msgsForwardedValue[i];
            Message storage message = messages[messageRef.shardId][messageRef.messageIndex];
        
            console.log("_forwardGas value: to=%_, fee=%_", message.to, message.feeCredit);
            require(feeCredit >= message.feeCredit, "forwardGas: not enough feeCredit for ForwardValue");
            feeCredit -= message.feeCredit;
        }

        // Process PERCENTAGE forwarded messages
        uint percentageTotal = 0;
        uint baseFeeCredit = feeCredit;
        for (uint256 i = 0; i < msgsForwardedPercentage.length; i++) {
            MessageRef memory messageRef = msgsForwardedPercentage[i];
            Message storage message = messages[messageRef.shardId][messageRef.messageIndex];
            
            require(message.forwardKind == Nil.FORWARD_PERCENTAGE, "forwardGas: invalid percentage forwarding");

            percentageTotal += message.feeCredit;
            if (percentageTotal > 100) {
                revert("forwardGas: total percentage is greater than 100");
            }

            message.feeCredit = (message.feeCredit * baseFeeCredit) / 100;

            if (feeCredit < message.feeCredit) {
                message.feeCredit = feeCredit;
                feeCredit = 0;
            } else {
                feeCredit -= message.feeCredit;
            }

            console.log("_forwardGas percentage: fee=%_", message.feeCredit);
        }

        // Process REMAINING forwarded messages
        if (msgsForwardedRemaining.length != 0) {
            if (feeCredit == 0) {
                revert("forwardGas: not enough feeCredit for ForwardRemaining");
            }
            uint feeCreditForward = feeCredit / msgsForwardedRemaining.length;
            feeCredit = 0;
            
            for (uint256 i = 0; i < msgsForwardedRemaining.length; i++) {
                MessageRef memory messageRef = msgsForwardedRemaining[i];
                Message storage message = messages[messageRef.shardId][messageRef.messageIndex];

                console.log("_forwardGas remaining: to=%_, fee=%_", message.to, feeCreditForward);

                require(message.forwardKind == Nil.FORWARD_REMAINING);
                message.feeCredit = feeCreditForward;
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
     * @dev Processes the forwarding kind of a message.
     * @param forwardKind The forwarding type.
     * @param messageRef The reference to the message.
     */
    function processForwardKind(
        uint8 forwardKind,
        MessageRef memory messageRef
    ) internal {
        if (forwardKind == Nil.FORWARD_REMAINING) {
            msgsForwardedRemaining.push(messageRef);
            numMsgsWithForwardGas++;
            console.log("processForwardKind FORWARD_REMAINING: num=%_", numMsgsWithForwardGas);
        } else if (forwardKind == Nil.FORWARD_PERCENTAGE) {
            msgsForwardedPercentage.push(messageRef);
            numMsgsWithForwardGas++;
            console.log("processForwardKind FORWARD_PERCENTAGE: num=%_", numMsgsWithForwardGas);
        } else if (forwardKind == Nil.FORWARD_VALUE) {
            msgsForwardedValue.push(messageRef);
            numMsgsWithForwardGas++;
            console.log("processForwardKind FORWARD_VALUE: num=%_", numMsgsWithForwardGas);
        } else if (forwardKind == Nil.FORWARD_NONE) {
            numMsgsWithForwardGas++;
        } else {
            revert("processForwardKind: invalid forwardKind");
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
        console.log("sendTx: to=%_, from=%_", to, msg.sender);

        require(forwardingInitialized, "Relayer not initialized");

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
        MessageRef memory messageRef = enqueueMessage(
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

        processForwardKind(forwardKind, messageRef);

        console.log("sendTx done: shardId=%_, messageId=%_", messageRef.shardId, messageRef.messageIndex);
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
        MessageRef memory messageRef = enqueueMessage(
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

        processForwardKind(forwardKind, messageRef);
        
        console.log("sendTxDeploy pushed: shardId=%_, messageId=%_", messageRef.shardId, messageRef.messageIndex);
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
        uint64 messageId,
        address bounceTo,
        uint value,
        Nil.Token[] memory tokens,
        bytes memory callData,
        uint256 requestId,
        uint responseFeeCredit
    ) public payable returns(bytes memory) {
        require(inMsgCount[uint32(Nil.getShardId(from))]++ == messageId, "Invalid message ID");

        console.log("receiveTx: gas=%_, to=%_, from=%_, messageId=%_", gasleft(), to, from, messageId);

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
        uint64 messageId,
        address bounceTo,
        uint256 value,
        uint256 salt,
        bytes memory code
    ) public payable returns(bytes memory) {
        require(inMsgCount[uint32(Nil.getShardId(from))]++  == messageId, "Invalid message ID");

        console.log("receiveTxDeploy: gas=%_, to=%_, salt=%_, messageId=%_", gasleft(), to, salt, messageId);

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
        uint64 messageId,
        uint256 value,
        bool success,
        bytes memory response,
        uint256 requestId,
        uint256 responseFeeCredit
    ) public payable {
        require(inMsgCount[uint32(Nil.getShardId(from))]++ == messageId, "Invalid message ID");
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
        address from,
        address to,
        uint64 messageId,
        uint value,
        Nil.Token[] memory tokens,
        bytes memory callData
    ) public payable {
        require(inMsgCount[uint32(Nil.getShardId(from))]++ == messageId, "Invalid message ID");
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
}
