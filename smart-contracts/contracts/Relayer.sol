// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./NilTokenManager.sol";
import "./MessageQueue.sol";

/**
 * @title Relayer
 * @dev Handles message execution and routing between shards
 */
contract Relayer {
    // Message Queue contract
    address public messageQueueAddress;
    
    // Admin address
    address public admin;
    
    // Events
    event MessageExecuted(
        bytes32 indexed messageId,
        address indexed from,
        address indexed to,
        bool success,
        uint256 gasUsed
    );
    
    event ResponseProcessingFailed(
        bytes32 indexed messageId,
        address from,
        address to,
        uint256 requestId,
        uint256 gasUsed
    );
    
    event BounceExecuted(
        bytes32 indexed messageId,
        address to,
        bool success
    );

    /**
     * @dev Set up the relayer with message queue and admin
     */
    constructor(address _messageQueueAddress, address _admin) {
        messageQueueAddress = _messageQueueAddress;
        admin = _admin;
    }
    
    modifier onlyAdmin() {
        require(msg.sender == admin, "Relayer: caller is not admin");
        _;
    }
    
    modifier onlyMessageQueue() {
        require(msg.sender == messageQueueAddress, "Relayer: caller is not message queue");
        _;
    }
    
    /**
     * @dev Update the MessageQueue address
     */
    function setMessageQueueAddress(address _messageQueueAddress) external onlyAdmin {
        messageQueueAddress = _messageQueueAddress;
    }
    
    /**
     * @dev Execute a message from the message queue
     * @param messageId ID of the message to execute
     * @param sourceChain Chain ID where message originated
     * @param from Address that sent the message
     * @param to Destination address
     * @param bounceTo Address to bounce to if execution fails
     * @param value Value to send with the call
     * @param tokens Tokens to send with the call
     * @param data Calldata for the execution
     * @param requestId ID for request/response pattern
     * @param responseFeeCredit Fee credit for response processing
     * @return success Whether the execution was successful
     * @return returnData Data returned from the execution
     */
    function executeMessage(
        bytes32 messageId,
        uint256 sourceChain,
        address from,
        address to,
        address bounceTo,
        uint256 value,
        Nil.Token[] memory tokens,
        bytes memory data,
        uint256 requestId,
        uint256 responseFeeCredit
    ) external payable onlyMessageQueue returns (bool success, bytes memory returnData) {
        // Credit tokens to the recipient
        if (tokens.length > 0) {
            NilTokenManager(Nil.getTokenManagerAddress()).creditForRelay(to, tokens);
        }
        
        // Measure gas before execution
        uint256 gasStart = gasleft();
        
        // Execute the call
        (success, returnData) = to.call{value: value}(data);
        
        // Calculate gas used
        uint256 gasUsed = gasStart - gasleft();
        
        // Reset transaction tokens
        NilTokenManager(Nil.getTokenManagerAddress()).resetTxTokens();
        
        // Emit event for tracking
        emit MessageExecuted(messageId, from, to, success, gasUsed);
        
        // Mark message as delivered in message queue
        MessageQueue(messageQueueAddress).messageDelivered(messageId, success, gasUsed);
        
        // Handle request/response pattern
        if (requestId != 0) {
            require(responseFeeCredit > 0, "Relayer: response fee credit is required for request/response pattern");
            // Queue a response message
            uint256 returnValue = success ? 0 : value;
            
            MessageQueue(messageQueueAddress).queueResponse{value: returnValue}(
                messageId,
                sourceChain,
                to,
                from,
                returnValue,
                success,
                returnData,
                requestId,
                responseFeeCredit
            );
        } else if (!success) {
            // Handle bounce for failed messages
            if (tokens.length > 0) {
                // Recover tokens for bounce
                NilTokenManager(Nil.getTokenManagerAddress()).deductForRelay(to, address(this), tokens);
            }
            
            // Queue bounce message
            MessageQueue(messageQueueAddress).queueBounce{value: value}(
                sourceChain,
                bounceTo == address(0) ? from : bounceTo,
                value,
                tokens,
                returnData
            );
        }
        
        return (success, returnData);
    }
    
    /**
     * @dev Execute a response message
     * @param messageId ID of the response message
     * @param from Address that sent the response
     * @param to Destination address
     * @param value Value to send with the call
     * @param success Whether the original request succeeded
     * @param responseData Data returned from the original request
     * @param requestId ID of the original request
     * @param responseFeeCredit Fee credit for response processing
     */
    function executeResponse(
        bytes32 messageId,
        address from,
        address to,
        uint256 value,
        bool success,
        bytes memory responseData,
        uint256 requestId,
        uint256 responseFeeCredit
    ) external payable onlyMessageQueue returns (bool) {
        // Calculate gas to use for response processing
        uint256 gasPrice = Nil.getGasPrice(address(this));
        uint256 gas = (responseFeeCredit / gasPrice);
        
        // Call onFallback method on the destination
        bytes memory data = abi.encodeWithSignature(
            "onFallback(uint256,bool,bytes)",
            requestId,
            success,
            responseData
        );
        
        uint256 gasStart = gasleft();
        (bool s, ) = to.call{gas: gas, value: value}(data);
        uint256 gasUsed = gasStart - gasleft();
        
        // Emit event for response processing
        if (!s) {
            emit ResponseProcessingFailed(messageId, from, to, requestId, gasUsed);
        }
        
        // Mark response as delivered
        MessageQueue(messageQueueAddress).messageDelivered(messageId, s, gasUsed);
        
        return s;
    }
    
    /**
     * @dev Execute a bounce message
     * @param messageId ID of the bounce message
     * @param to Address to bounce to
     * @param value Value to return
     * @param tokens Tokens to return
     * @param errorData Error data from the failed call
     */
    function executeBounce(
        bytes32 messageId,
        address to,
        uint256 value,
        Nil.Token[] memory tokens,
        bytes memory errorData
    ) external payable onlyMessageQueue returns (bool) {
        // Credit tokens back to the original sender
        if (tokens.length > 0) {
            NilTokenManager(Nil.getTokenManagerAddress()).creditForRelay(to, tokens);
        }
        
        // Call bounce method if it exists
        bytes memory data = abi.encodeWithSignature("bounce(bytes)", errorData);
        (bool success, ) = to.call{value: value}(data);
        
        // Reset transaction tokens
        NilTokenManager(Nil.getTokenManagerAddress()).resetTxTokens();
        
        // Emit event for bounce execution
        emit BounceExecuted(messageId, to, success);
        
        // Mark bounce as delivered
        MessageQueue(messageQueueAddress).messageDelivered(messageId, success, 0);
        
        return success;
    }
}
