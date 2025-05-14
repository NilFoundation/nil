// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import { IBridgeMessenger } from "../../interfaces/IBridgeMessenger.sol";
import { NilConstants } from "../../../common/libraries/NilConstants.sol";
import { IRelayMessage } from "./IRelayMessage.sol";

/// @title IL1BridgeMessenger
/// @notice Interface for the L1BridgeMessenger contract which handles cross-chain messaging between L1 and L2.
/// @dev This interface defines the functions and events for managing deposit messages, sending messages, and canceling
/// deposits.
interface IL1BridgeMessenger is IBridgeMessenger, IRelayMessage {
  /*//////////////////////////////////////////////////////////////////////////
                             ERRORS
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Thrown when a deposit message already exists.
  error DepositMessageAlreadyExist();

  /// @notice Thrown when a deposit message does not exist.
  error DepositMessageDoesNotExist();

  /// @notice Thrown when a deposit message is already cancelled.
  error DepositMessageAlreadyCancelled();

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

  error ErrorInvalidMessageSender();

  error ErrorInvalidMessageTarget();

  error ErrorDuplicateWithdrawalClaim();

  error ErrorInvalidMessageHash();

  error ErrorFailedWithdrawalClaim();

  error ErrorInvalidMessageType();

  /*//////////////////////////////////////////////////////////////////////////
                             EVENTS
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Emitted when a deposit message is cancelled.
  /// @param messageHash The hash of the deposit message that was cancelled.
  event DepositMessageCancelled(bytes32 messageHash);

  event WithdrawalClaimed(
    bytes32 indexed withdrawalMessageHash,
    uint256 withdrawalMessageNonce,
    uint256 merkleLeafIndex
  );

  /*//////////////////////////////////////////////////////////////////////////
                             MESSAGE STRUCTS
    //////////////////////////////////////////////////////////////////////////*/

  struct AddressSlot {
    address value;
  }

  struct WithdrawalRequestParams {
    NilConstants.MessageType messageType;
    address messageSender;
    address messageTarget;
    uint256 messageNonce;
    uint256 merkleLeafIndex;
    bytes message;
    bytes32 messageHash;
    bytes32[] withdrawalProof;
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
  function getDepositMessage(bytes32 msgHash) external view returns (DepositMessage memory depositMessage);

  /// @notice Get the list of authorized bridges
  /// @return The list of authorized bridge addresses.
  function getAuthorizedBridges() external view returns (address[] memory);

  function computeWithdrawalMessageHash(
    NilConstants.MessageType messageType,
    address messageSender,
    address messageTarget,
    uint256 messageNonce,
    bytes memory message
  ) external pure returns (bytes32);

  function computeDepositMessageHash(
    NilConstants.MessageType messageType,
    address messageSender,
    address messageTarget,
    uint256 messageNonce,
    bytes memory message
  ) external pure returns (bytes32);

  /*//////////////////////////////////////////////////////////////////////////
                           PUBLIC MUTATING FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Cancels a deposit message.
  /// @param messageHash The hash of the deposit message to cancel.
  function cancelDeposit(bytes32 messageHash) external;

  function claimFailedDeposit(bytes32 messageHash, uint256 merkleTreeLeafNonce, bytes32[] memory claimProof) external;

  function claimWithdrawal(WithdrawalRequestParams calldata withdrawalRequestParams) external;

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

  function getQueueHeadIndex() external view returns (uint256);
}
