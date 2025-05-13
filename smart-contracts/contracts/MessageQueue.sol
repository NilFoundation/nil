// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./Nil.sol";
import "./NilTokenManager.sol";

/**
 * @title MessageQueue
 * @dev Manages cross-shard message passing with pruning support
 */
contract MessageQueue is NilBase {
    // Message statuses
    uint8 public constant STATUS_PENDING = 0;
    uint8 public constant STATUS_DELIVERED = 1;
    uint8 public constant STATUS_FAILED = 2;
    uint8 public constant STATUS_PRUNED = 3;

    // Message types
    uint8 public constant TYPE_REGULAR = 0;
    uint8 public constant TYPE_RESPONSE = 1;
    uint8 public constant TYPE_BOUNCE = 2;

    // Forwarding kinds (matching the existing system)
    uint8 public constant FORWARD_REMAINING = 0;
    uint8 public constant FORWARD_PERCENTAGE = 1;
    uint8 public constant FORWARD_VALUE = 2;
    uint8 public constant FORWARD_NONE = 3;

    struct Message {
        bytes32 id;                // Unique message identifier
        uint8 messageType;         // Type of message (regular, response, bounce)
        uint8 status;              // Current status of the message
        uint256 sourceChain;       // Chain ID of the source
        uint256 destinationChain;  // Chain ID of the destination
        address from;              // Address that sent the message
        address to;                // Destination address
        address refundTo;          // Address to refund if operation fails
        address bounceTo;          // Address to bounce to if fails
        uint256 value;             // Value sent with the message
        uint8 forwardKind;         // Forwarding kind for gas
        uint256 feeCredit;         // Fee credit for the transaction
        Nil.Token[] tokens;        // Tokens to send with the message
        bytes data;                // Calldata for the transaction
        uint256 requestId;         // ID for request (for response tracking)
        uint256 responseGas;       // Gas reserved for response processing
        uint256 timestamp;         // Block timestamp when message was queued
        uint256 nonce;             // Sender's nonce for this message
        uint256 blockNumber;       // Block number when message was queued
    }

    // Outbound queue structure
    struct OutboundQueue {
        bytes32[] messageIds;         // Array of message IDs in order
        uint256 firstIndex;           // Index of the first unprocessed message
        mapping(bytes32 => Message) messages; // Message data by ID
        mapping(bytes32 => uint256) indices;  // Index of each message ID in the array
    }

    // Queue of outbound messages per destination chain
    mapping(uint256 => OutboundQueue) public outboundQueues;
    
    // Track processed messages to prevent duplicates
    mapping(bytes32 => bool) public processedMessages;
    
    // Track the last pruned block for each destination
    mapping(uint256 => uint256) public lastPrunedBlock;
    
    // Admin address that can trigger pruning
    address public admin;
    
    // Relayer address that can deliver messages
    address public relayerAddress;
    
    // This chain's ID
    uint256 public chainId;
    
    // Track async call contexts
    struct AsyncSession {
        bytes32[] messageIds;
        uint256 totalGas;
        uint256 totalValue;
    }
    
    mapping(address => mapping(uint256 => AsyncSession)) public asyncSessions;
    mapping(address => uint256) public currentSessionIds;
    mapping(bytes32 => bytes32) public responseMapping; // originalMessageId => responseMessageId

    // Events
    event ResponseQueued(
        bytes32 indexed responseId,
        bytes32 indexed originalMessageId,
        address indexed recipient,
        bool success
    );

    event MessageQueued(
        bytes32 indexed id,
        uint256 indexed destinationChain,
        address indexed from,
        address to,
        uint256 value,
        bool isDeploy,
        uint256 requestId,
        uint8 messageType
    );
    
    event MessageDelivered(
        bytes32 indexed id,
        bool success,
        uint256 gasUsed
    );
    
    event MessagesPruned(
        uint256 indexed destinationChain,
        uint256 count,
        uint256 upToBlock
    );
    
    event AsyncSessionStarted(
        address indexed caller,
        uint256 sessionId
    );
    
    event AsyncSessionFinalized(
        address indexed caller,
        uint256 sessionId,
        uint256 messageCount,
        uint256 gasPerMessage
    );

    constructor(address _admin, uint256 _chainId) {
        admin = _admin;
        chainId = _chainId;
    }
    
    modifier onlyAdmin() {
        require(msg.sender == admin, "MessageQueue: caller is not admin");
        _;
    }
    
    modifier onlyRelayer() {
        require(msg.sender == relayerAddress, "MessageQueue: caller is not relayer");
        _;
    }
    
    /**
     * @dev Set the relayer address
     * @param _relayerAddress The new relayer address
     */
    function setRelayerAddress(address _relayerAddress) external onlyAdmin {
        relayerAddress = _relayerAddress;
    }
    
    /**
     * @dev Generates a unique message ID
     */
    function generateMessageId(
        address from,
        uint256 nonce,
        address to,
        uint256 timestamp,
        uint256 destinationChain
    ) public pure returns (bytes32) {
        return keccak256(abi.encodePacked(from, nonce, to, timestamp, destinationChain));
    }
    
    /**
     * @dev Start a new async session
     */
    function startAsync() external {
        uint256 sessionId = currentSessionIds[msg.sender];
        asyncSessions[msg.sender][sessionId] = AsyncSession({
            messageIds: new bytes32[](0),
            totalGas: 0,
            totalValue: 0
        });
        
        emit AsyncSessionStarted(msg.sender, sessionId);
    }
    
    /**
     * @dev Finalize an async session by allocating gas equally to all messages
     * @param totalGas Total gas to allocate across all messages
     */
    function finalizeAsync(uint256 totalGas) external payable {
        uint256 sessionId = currentSessionIds[msg.sender];
        AsyncSession storage session = asyncSessions[msg.sender][sessionId];
        
        require(session.messageIds.length > 0, "MessageQueue: no messages in session");
        
        uint256 gasPerMessage = totalGas / session.messageIds.length;
        uint256 requiredValue = session.totalValue;
        require(msg.value >= requiredValue, "MessageQueue: insufficient value");
        
        // Update all messages with gas allocation
        for (uint256 i = 0; i < session.messageIds.length; i++) {
            bytes32 messageId = session.messageIds[i];
            uint256 destinationChain = extractDestinationChain(messageId);
            Message storage message = outboundQueues[destinationChain].messages[messageId];
            message.feeCredit = gasPerMessage * tx.gasprice;
        }
        
        // Increment session ID for next async session
        currentSessionIds[msg.sender]++;
        
        emit AsyncSessionFinalized(msg.sender, sessionId, session.messageIds.length, gasPerMessage);
    }
    
    /**
     * @dev Extract destination chain from a message ID (helper function)
     */
    function extractDestinationChain(bytes32 messageId) internal view returns (uint256) {
        // First try to find it in any outbound queue
        for (uint256 i = 0; i < Nil.SHARDS_NUM; i++) {
            if (outboundQueues[i].indices[messageId] > 0) {
                return i;
            }
        }
        
        // If not found, default to 0
        return 0;
    }
    
    /**
     * @dev Queue a message for cross-shard execution
     */
    function queueMessage(
        bool isDeploy,
        uint8 forwardKind,
        address to,
        address refundTo,
        address bounceTo,
        uint256 feeCredit,
        uint256 value,
        Nil.Token[] memory tokens,
        bytes memory data,
        uint256 requestId,
        uint256 responseGas
    ) public payable returns (bytes32) {
        // Validate destination
        uint256 destinationChain = Nil.getShardId(to);
        require(destinationChain != chainId, "MessageQueue: cannot send to same chain");
        require(destinationChain < Nil.SHARDS_NUM, "MessageQueue: invalid destination chain");
        
        // For requests, ensure enough gas for response
        if (requestId != 0) {
            require(responseGas >= 50000, "MessageQueue: insufficient response gas");
        }
        
        // Generate unique ID
        uint256 nonce = outboundQueues[destinationChain].messageIds.length;
        bytes32 messageId = generateMessageId(
            msg.sender,
            nonce,
            to,
            block.timestamp,
            destinationChain
        );
        
        // Create message
        Message memory message = Message({
            id: messageId,
            messageType: TYPE_REGULAR,
            status: STATUS_PENDING,
            sourceChain: chainId,
            destinationChain: destinationChain,
            from: msg.sender,
            to: to,
            refundTo: refundTo == address(0) ? msg.sender : refundTo,
            bounceTo: bounceTo == address(0) ? msg.sender : bounceTo,
            value: value,
            forwardKind: forwardKind,
            feeCredit: feeCredit,
            tokens: tokens,
            data: data,
            requestId: requestId,
            responseGas: responseGas,
            timestamp: block.timestamp,
            nonce: nonce,
            blockNumber: block.number
        });
        
        // Add to outbound queue
        OutboundQueue storage queue = outboundQueues[destinationChain];
        queue.messageIds.push(messageId);
        queue.messages[messageId] = message;
        queue.indices[messageId] = queue.messageIds.length;
        
        // If we're in an async session, add to the session
        if (currentSessionIds[msg.sender] > 0) {
            asyncSessions[msg.sender][currentSessionIds[msg.sender]].messageIds.push(messageId);
            asyncSessions[msg.sender][currentSessionIds[msg.sender]].totalValue += value;
        } else {
            // Handle tokens for immediate send
            if (tokens.length > 0) {
                NilTokenManager(Nil.getTokenManagerAddress()).deductForRelay(
                    msg.sender,
                    to,
                    tokens
                );
            }
        }
        
        emit MessageQueued(messageId, destinationChain, msg.sender, to, value, isDeploy, requestId, TYPE_REGULAR);
        return messageId;
    }
    
    /**
     * @dev Queue a response message
     */
    function queueResponse(
        bytes32 originalMessageId,
        uint256 destinationChain,
        address from,
        address to,
        uint256 value,
        bool success,
        bytes memory responseData,
        uint256 requestId,
        uint256 responseFeeCredit
    ) external payable returns (bytes32) {
        require(msg.sender == relayerAddress, "MessageQueue: only relayer can queue responses");
        
        // Generate unique ID for response
        uint256 nonce = outboundQueues[destinationChain].messageIds.length;
        bytes32 messageId = generateMessageId(
            from,
            nonce,
            to,
            block.timestamp,
            destinationChain
        );
        
        // Create response message
        Message memory message = Message({
            id: messageId,
            messageType: TYPE_RESPONSE,
            status: STATUS_PENDING,
            sourceChain: chainId,
            destinationChain: destinationChain,
            from: from,
            to: to,
            refundTo: address(0),
            bounceTo: address(0),
            value: value,
            forwardKind: FORWARD_REMAINING,
            feeCredit: responseFeeCredit,
            tokens: new Nil.Token[](0),
            data: abi.encode(success, responseData, requestId, originalMessageId), // Include originalMessageId in the data
            requestId: requestId,
            responseGas: 0,
            timestamp: block.timestamp,
            nonce: nonce,
            blockNumber: block.number
        });
        
        // Add to outbound queue
        OutboundQueue storage queue = outboundQueues[destinationChain];
        queue.messageIds.push(messageId);
        queue.messages[messageId] = message;
        queue.indices[messageId] = queue.messageIds.length;
        
        // Add a mapping from original message ID to response message ID for tracking
        responseMapping[originalMessageId] = messageId;
        
        emit MessageQueued(messageId, destinationChain, from, to, value, false, requestId, TYPE_RESPONSE);
        emit ResponseQueued(messageId, originalMessageId, to, success); // Add a specific event for responses
        
        return messageId;
    }    
    /**
     * @dev Queue a bounce message
     */
    function queueBounce(
        uint256 destinationChain,
        address to,
        uint256 value,
        Nil.Token[] memory tokens,
        bytes memory errorData
    ) external payable returns (bytes32) {
        require(msg.sender == relayerAddress, "MessageQueue: only relayer can queue bounces");
        
        // Generate unique ID for bounce
        uint256 nonce = outboundQueues[destinationChain].messageIds.length;
        bytes32 messageId = generateMessageId(
            address(this),
            nonce,
            to,
            block.timestamp,
            destinationChain
        );
        
        // Create bounce message
        Message memory message = Message({
            id: messageId,
            messageType: TYPE_BOUNCE,
            status: STATUS_PENDING,
            sourceChain: chainId,
            destinationChain: destinationChain,
            from: address(this),
            to: to,
            refundTo: address(0),
            bounceTo: address(0),
            value: value,
            forwardKind: FORWARD_REMAINING,
            feeCredit: 0,
            tokens: tokens,
            data: errorData,
            requestId: 0,
            responseGas: 0,
            timestamp: block.timestamp,
            nonce: nonce,
            blockNumber: block.number
        });
        
        // Add to outbound queue
        OutboundQueue storage queue = outboundQueues[destinationChain];
        queue.messageIds.push(messageId);
        queue.messages[messageId] = message;
        queue.indices[messageId] = queue.messageIds.length;
        
        emit MessageQueued(messageId, destinationChain, address(this), to, value, false, 0, TYPE_BOUNCE);
        return messageId;
    }
    
    /**
     * @dev Mark a message as delivered
     * @param messageId ID of the message that was delivered
     * @param success Whether the delivery was successful
     * @param gasUsed Amount of gas used for delivery
     */
    function messageDelivered(bytes32 messageId, bool success, uint256 gasUsed) external onlyRelayer {
        require(!processedMessages[messageId], "MessageQueue: message already processed");
        
        // Find the message in the appropriate queue
        for (uint256 i = 0; i < Nil.SHARDS_NUM; i++) {
            OutboundQueue storage queue = outboundQueues[i];
            uint256 index = queue.indices[messageId];
            
            if (index > 0) {
                // Mark as delivered
                queue.messages[messageId].status = success ? STATUS_DELIVERED : STATUS_FAILED;
                processedMessages[messageId] = true;
                
                emit MessageDelivered(messageId, success, gasUsed);
                return;
            }
        }
        
        revert("MessageQueue: message not found");
    }
    
    /**
     * @dev Prune processed messages across all destination chains
     * @return prunedCounts Array of pruned message counts per destination
     */
    function pruneAll() external onlyAdmin returns (uint256[] memory prunedCounts) {
        prunedCounts = new uint256[](Nil.SHARDS_NUM);
        
        // Process each destination chain
        for (uint256 destChain = 0; destChain < Nil.SHARDS_NUM; destChain++) {
            // Skip pruning for our own shard (no messages sent to self)
            if (destChain == chainId) continue;
            
            OutboundQueue storage queue = outboundQueues[destChain];
            
            // Skip empty queues
            if (queue.firstIndex >= queue.messageIds.length) continue;
            
            // Count how many messages we can prune for this destination
            uint256 prunedCount = 0;
            uint256 i = queue.firstIndex;
            uint256 maxMessages = 50; // Limit per destination to prevent excessive gas usage
            uint256 lastBlockPruned = 0;
            
            while (i < queue.messageIds.length && prunedCount < maxMessages) {
                // Check remaining gas to avoid out-of-gas errors
                if (gasleft() < 100000) break; // Break early if gas is running low
                
                bytes32 messageId = queue.messageIds[i];
                Message storage message = queue.messages[messageId];
                
                // Prune any message that has been processed (delivered or failed)
                if (message.status == STATUS_DELIVERED || message.status == STATUS_FAILED) {
                    if (message.blockNumber > lastBlockPruned) {
                        lastBlockPruned = message.blockNumber;
                    }
                    
                    message.status = STATUS_PRUNED;
                    delete queue.messages[messageId];
                    prunedCount++;
                    i++;
                } else {
                    // Stop at first non-prunable message
                    break;
                }
            }
            
            // Update the first index
            if (prunedCount > 0) {
                queue.firstIndex += prunedCount;
                prunedCounts[destChain] = prunedCount;
                
                // Update the last pruned block if we pruned something
                if (lastBlockPruned > 0) {
                    lastPrunedBlock[destChain] = lastBlockPruned;
                }
                
                emit MessagesPruned(destChain, prunedCount, lastBlockPruned);
            }
        }
        
        return prunedCounts;
    }
    
    /**
     * @dev Get pending messages for a specific destination
     * @param destinationChain Chain ID to get messages for
     * @param maxCount Maximum number of messages to return
     * @return messages Array of pending messages
     */
    function getPendingMessages(uint256 destinationChain, uint256 maxCount) 
        external 
        view 
        returns (Message[] memory) 
    {
        require(destinationChain < Nil.SHARDS_NUM, "MessageQueue: invalid destination chain");
        
        OutboundQueue storage queue = outboundQueues[destinationChain];
        
        // Count pending messages
        uint256 pendingCount = 0;
        for (uint256 i = queue.firstIndex; i < queue.messageIds.length && pendingCount < maxCount; i++) {
            bytes32 messageId = queue.messageIds[i];
            if (queue.messages[messageId].status == STATUS_PENDING) {
                pendingCount++;
            }
        }
        
        // Create array of pending messages
        Message[] memory pendingMessages = new Message[](pendingCount);
        uint256 resultIndex = 0;
        
        for (uint256 i = queue.firstIndex; i < queue.messageIds.length && resultIndex < pendingCount; i++) {
            bytes32 messageId = queue.messageIds[i];
            if (queue.messages[messageId].status == STATUS_PENDING) {
                pendingMessages[resultIndex] = queue.messages[messageId];
                resultIndex++;
            }
        }
        
        return pendingMessages;
    }
    
    /**
     * @dev Get message details by ID
     * @param messageId ID of the message to retrieve
     * @return message The message details
     */
    function getMessage(bytes32 messageId) external view returns (Message memory) {
        for (uint256 i = 0; i < Nil.SHARDS_NUM; i++) {
            OutboundQueue storage queue = outboundQueues[i];
            uint256 index = queue.indices[messageId];
            
            if (index > 0) {
                return queue.messages[messageId];
            }
        }
        
        revert("MessageQueue: message not found");
    }
    
    /**
     * @dev Get queue statistics for a destination
     * @param destinationChain Chain ID to get stats for
     * @return total Total number of messages ever queued
     * @return pending Number of pending messages
     * @return firstIndex Index of first unprocessed message
     * @return lastPruned Last block number that was pruned
     */
    function getQueueStats(uint256 destinationChain) 
        external 
        view 
        returns (uint256 total, uint256 pending, uint256 firstIndex, uint256 lastPruned) 
    {
        require(destinationChain < Nil.SHARDS_NUM, "MessageQueue: invalid destination chain");
        
        OutboundQueue storage queue = outboundQueues[destinationChain];
        
        // Count pending messages
        uint256 pendingCount = 0;
        for (uint256 i = queue.firstIndex; i < queue.messageIds.length; i++) {
            bytes32 messageId = queue.messageIds[i];
            if (queue.messages[messageId].status == STATUS_PENDING) {
                pendingCount++;
            }
        }
        
        return (
            queue.messageIds.length,
            pendingCount,
            queue.firstIndex,
            lastPrunedBlock[destinationChain]
        );
    }
    
    /**
     * @dev Update the admin address
     * @param newAdmin New admin address
     */
    function setAdmin(address newAdmin) external onlyAdmin {
        require(newAdmin != address(0), "MessageQueue: invalid admin address");
        admin = newAdmin;
    }
}
