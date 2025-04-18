// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { IBridgeMessenger } from "../../interfaces/IBridgeMessenger.sol";
import { NilConstants } from "../../../common/libraries/NilConstants.sol";
import { L1BridgeMessengerEvents } from "../../libraries/L1BridgeMessengerEvents.sol";

/// @title IL1BridgeMessenger
/// @notice Interface for the L1BridgeMessenger contract which handles cross-chain messaging between L1 and L2.
/// @dev This interface defines the functions and events for managing deposit messages, sending messages, and canceling
/// deposits.
interface IL1BridgeMessenger is IBridgeMessenger {
  /*//////////////////////////////////////////////////////////////////////////
                             ERRORS
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Thrown when a deposit message already exists.
  /// @param messageHash The hash of the deposit message.
  error DepositMessageAlreadyExist(bytes32 messageHash);

  /// @notice Thrown when a deposit message does not exist.
  /// @param messageHash The hash of the deposit message.
  error DepositMessageDoesNotExist(bytes32 messageHash);

  /// @notice Thrown when a deposit message is already cancelled.
  /// @param messageHash The hash of the deposit message.
  error DepositMessageAlreadyCancelled(bytes32 messageHash);

  /// @notice Thrown when a deposit message is not expired.
  /// @param messageHash The hash of the deposit message.
  error DepositMessageNotExpired(bytes32 messageHash);

  /// @notice Thrown when a message hash is not in the queue.
  /// @param messageHash The hash of the deposit message.
  error MessageHashNotInQueue(bytes32 messageHash);

  /// @notice Thrown when the max message processing time is invalid.
  error InvalidMaxMessageProcessingTime();

  /// @notice Thrown when the message cancel delta time is invalid.
  error InvalidMessageCancelDeltaTime();

  /// @notice Thrown when a bridge interface is invalid.
  error InvalidBridgeInterface();

  /// @notice Thrown when a bridge is already authorized.
  error BridgeAlreadyAuthorized();

  /// @notice Thrown when a bridge is not authorized.
  error BridgeNotAuthorized();

  /// @notice Thrown when any address other than l1NilRollup is attempting to remove messages from queue
  error NotAuthorizedToPopMessages();

  /// @notice Thrown when the deposit message has already been claimed.
  error DepositMessageAlreadyClaimed();

  /// @notice  Thrown when the deposit message hash is still in the queue, indicating that the message has not been
  /// executed on L2.
  error DepositMessageStillInQueue();

   /*//////////////////////////////////////////////////////////////////////////
                             MESSAGE STRUCTS   
    //////////////////////////////////////////////////////////////////////////*/

  struct AddressSlot {
    address value;
  }

  /// @notice Gets the current deposit nonce.
  /// @return The current deposit nonce.
  function depositNonce() external view returns (uint256);

  /// @notice Gets the next deposit nonce.
  /// @return The next deposit nonce.
  function getNextDepositNonce() external view returns (uint256);

  /// @notice Gets the deposit type for a given message hash.
  /// @param msgHash The hash of the deposit message.
  /// @return messageType The type of the message.
  function getMessageType(bytes32 msgHash) external view returns (NilConstants.MessageType messageType);

  /// @notice Gets the deposit message for a given message hash.
  /// @param msgHash The hash of the deposit message.
  /// @return depositMessage The deposit message details.
  function getDepositMessage(bytes32 msgHash) external view returns (L1BridgeMessengerEvents.DepositMessage memory depositMessage);

  /// @notice Get the list of authorized bridges
  /// @return The list of authorized bridge addresses.
  function getAuthorizedBridges() external view returns (address[] memory);

  function computeMessageHash(
    address messageSender,
    address messageTarget,
    uint256 messageNonce,
    bytes memory message
  ) external pure returns (bytes32);

  /*//////////////////////////////////////////////////////////////////////////
                           PUBLIC MUTATING FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Send cross chain message from L1 to L2 or L2 to L1.
  /// @param messageType The messageType enum value
  /// @param messageTarget The address of contract/account who receive the message.
  /// @param message The content of the message.
  /// @param l1DepositRefundAddress The address of recipient for the deposit-cancellation or claim failed deposit
  /// @param l2FeeRefundAddress The address of the feeRefundRecipient on L2.
  /// @param feeCreditData The feeCreditData for l2-Transaction-fee
  function sendMessage(
    NilConstants.MessageType messageType,
    address messageTarget,
    bytes calldata message,
    address tokenAddress,
    address depositorAddress,
    uint256 depositAmount,
    address l1DepositRefundAddress,
    address l2FeeRefundAddress,
    L1BridgeMessengerEvents.FeeCreditData memory feeCreditData
  ) external payable;

  /// @notice Cancels a deposit message.
  /// @param messageHash The hash of the deposit message to cancel.
  function cancelDeposit(bytes32 messageHash) external;

  function claimFailedDeposit(bytes32 messageHash, bytes32[] calldata claimProof) external;

  /*//////////////////////////////////////////////////////////////////////////
                           RESTRICTED FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  function setCounterpartyBridgeMessenger(address counterpartyBridgeMessengerAddress) external;

  /// @notice Authorize a bridge addresses
  /// @param bridges The array of addresses of the bridges to authorize.
  function authorizeBridges(address[] memory bridges) external;

  /// @notice Authorize a bridge address
  /// @param bridge The address of the bridge to authorize.
  function authorizeBridge(address bridge) external;

  /// @notice Revoke authorization of a bridge address
  /// @param bridge The address of the bridge to revoke.
  function revokeBridgeAuthorization(address bridge) external;

  /// @notice remove a list of messageHash values from the depositMessageQueue.
  /// @dev messages are always popped from the queue in FIFIO Order
  /// @param messageCount number of messages to be removed from the queue
  /// @return messageHashes array of messageHashes start from the head of queue
  function popMessages(uint256 messageCount) external returns (bytes32[] memory);
}
