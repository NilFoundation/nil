// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { IBridgeMessenger } from "../../interfaces/IBridgeMessenger.sol";
import { NilConstants } from "../../../common/libraries/NilConstants.sol";

/// @title IL2BridgeMessenger
/// @notice Interface for the L2BridgeMessenger contract which handles cross-chain messaging between L1 and L2.
/// @dev This interface defines the functions and events for finalizing deposit messages, sending messages to L1, and
/// initiating withdrawals
interface IL2BridgeMessenger is IBridgeMessenger {
  /*//////////////////////////////////////////////////////////////////////////
                             ERRORS
    //////////////////////////////////////////////////////////////////////////*/

  error ErrorInvalidRelayer();

  error BridgeNotAuthorized();

  error ErrorWithdrawalAlreadyInitiated();

  error ErrorDuplicateWithdrawalMessage(bytes32 messageHash);

  /*//////////////////////////////////////////////////////////////////////////
                             STRUCTS   
    //////////////////////////////////////////////////////////////////////////*/

  struct AddressSlot {
    address value;
  }

  struct SendMessageParams {
    NilConstants.MessageType messageType;
    address messageTarget;
    bytes message;
  }

  /**
   * @notice Represents a deposit message.
   * @dev The fields used for `messageHash` generation are:
   * - sender
   * - target
   * - value
   * - nonce
   * - message
   */
  struct WithdrawalMessage {
    address sender; // The address of the sender
    address target; // The target address on the destination chain
    uint256 nonce; // The nonce for the withdrawal
    uint256 creationTime; // The creation-time in epochSeconds
    NilConstants.MessageType messageType; // The type of the message
    bytes message; // The encoded message data generated by the bridge contract
  }

  /*//////////////////////////////////////////////////////////////////////////
                             EVENTS
    //////////////////////////////////////////////////////////////////////////*/

  event MessageExecutionFailed(bytes32 indexed messageHash);

  event MessageExecutionSuccessful(bytes32 indexed messageHash);

  event L2BridgeRouterSet(address indexed bridgeRouter, address indexed newBridgeRouter);

  event RelayerSet(address indexed relayer, address indexed relayerAddress);

  /// @notice Emitted when a message is sent.
  /// @param messageSender The address of the message sender.
  /// @param messageTarget The address of the message recipient which can be an account/smartcontract.
  /// @param messageNonce The nonce of the message.
  /// @param message The encoded message data.
  /// @param messageHash The hash of the message.
  /// @param messageType The type of the withdrawalMessage.
  /// @param messageCreatedAt The time at which message was recorded.
  /// from depositor
  event MessageSent(
    address indexed messageSender,
    address indexed messageTarget,
    uint256 indexed messageNonce,
    bytes message,
    bytes32 messageHash,
    NilConstants.MessageType messageType,
    uint256 messageCreatedAt
  );

  /*//////////////////////////////////////////////////////////////////////////
                         PUBLIC CONSTANT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Get the list of authorized bridges
  /// @return The list of authorized bridge addresses.
  function getAuthorisedBridges() external view returns (address[] memory);

  function isAuthorisedBridge(address bridgeAddress) external view returns (bool);

  function isFullyInitialised() external view returns (bool);

  function computeMessageHash(
    address messageSender,
    address messageTarget,
    uint256 messageNonce,
    bytes memory message
  ) external pure returns (bytes32);

  /// @notice Gets the current withdrawal nonce.
  /// @return The current withdrawal nonce.
  function withdrawalNonce() external view returns (uint256);

  /// @notice Gets the next withdrawal nonce.
  /// @return The next withdrawal nonce.
  function getNextWithdrawalNonce() external view returns (uint256);

  /// @notice Gets the withdrawal MessageType for a given message hash.
  /// @param msgHash The hash of the withdrawal message.
  /// @return messageType The type of the withdrawal message.
  function getMessageType(bytes32 msgHash) external view returns (NilConstants.MessageType messageType);

  /// @notice Gets the withdrawal message for a given message hash.
  /// @param msgHash The hash of the withdrawal message.
  /// @return withdrawalMessage The withdrawal message details.
  function getWithdrawalMessage(bytes32 msgHash) external view returns (WithdrawalMessage memory withdrawalMessage);

  /*//////////////////////////////////////////////////////////////////////////
                         PUBLIC MUTATION FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice receive realyedMessage originated from L1BridgeMessenger via Relayer
  /// @dev only authorized smart-account on nil-shard can relayMessage to Bridge on NilShard
  /// @param messageSender The address of the sender of the message.
  /// @param messageTarget The address of the recipient of the message.
  /// @param messageNonce The nonce of the message to avoid replay attack.
  /// @param message The content of the message.
  /// @param messageExpiryTime The expiryTime of message queued on L1.
  function relayMessage(
    address messageSender,
    address messageTarget,
    NilConstants.MessageType messageType,
    uint256 messageNonce,
    bytes calldata message,
    uint256 messageExpiryTime
  ) external;

  /// @notice Send cross chain message Nil to L1.
  /// @param messageType The type of withdrawalMessage.
  /// @param messageTarget The address of account who receive the message.
  /// @param message The content of the message.
  function sendMessage(
    NilConstants.MessageType messageType,
    address messageTarget,
    bytes memory message
  ) external payable returns (bytes32);

  /*//////////////////////////////////////////////////////////////////////////
                         OWNER RESTRICTED FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  function setCounterpartyBridgeMessenger(address counterpartyBridgeMessengerAddress) external;

  /// @notice Authorize a bridge addresses
  /// @param bridges The array of addresses of the bridges to authorize.
  function authoriseBridges(address[] memory bridges) external;

  /// @notice Authorize a bridge address
  /// @param bridge The address of the bridge to authorize.
  function authoriseBridge(address bridge) external;

  /// @notice Revoke authorization of a bridge address
  /// @param bridge The address of the bridge to revoke.
  function revokeBridgeAuthorisation(address bridge) external;

  /**
   * @notice Pauses or unpauses the contract.
   * @dev This function allows the owner to pause or unpause the contract.
   * @param _status The pause status to update.
   */
  function setPause(bool _status) external;
}
