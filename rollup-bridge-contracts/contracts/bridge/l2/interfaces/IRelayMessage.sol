// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { NilConstants } from "../../../common/libraries/NilConstants.sol";

/// @title IRelayMessage
/// @notice Interface for the L2BridgeMessenger contract which also used by relayer.
interface IRelayMessage {
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
}
