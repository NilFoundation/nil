// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import { NilConstants } from "../../../common/libraries/NilConstants.sol";

interface IRelayMessage {

  struct FeeCreditData {
    uint256 nilGasLimit;
    uint256 maxFeePerGas;
    uint256 maxPriorityFeePerGas;
    uint256 feeCredit;
  }

  struct SendMessageParams {
    NilConstants.MessageType messageType;
    address messageTarget;
    bytes message;
    address tokenAddress;
    address depositorAddress;
    uint256 depositAmount;
    address l1DepositRefundAddress;
    address l2FeeRefundAddress;
    FeeCreditData feeCreditData;
  }

  struct DepositMessage {
    address sender; // The address of the sender
    address target; // The target address on the destination chain
    uint256 nonce; // The nonce for the deposit
    uint256 creationTime; // The creation-time in epochSeconds
    uint256 expiryTime; // The expiry time for the deposit
    bool isCancelled; // Indicates if the deposit is cancelled
    bool isClaimed; // Indicates if the failed deposit is claimed
    address l1DepositRefundAddress; // The address to refund if the deposit is cancelled
    address l2FeeRefundAddress; // The address of the fee-refund recipient on NilChain
    NilConstants.MessageType messageType; // The type of the message
    address tokenAddress;
    address depositorAddress;
    uint256 depositAmount;
    FeeCreditData feeCreditData;
  }

  /// @notice Emitted when a message is sent.
  /// @param messageSender The address of the message sender.
  /// @param messageTarget The address of the message recipient which can be an account/smartcontract.
  /// @param messageNonce The nonce of the message.
  /// @param message The encoded message data.
  /// @param messageHash The hash of the message.
  /// @param messageType The type of the deposit.
  /// @param messageCreatedAt The time at which message was recorded.
  /// @param messageExpiryTime The expiry time of the message.
  /// @param l2FeeRefundAddress The address of the fee-refund recipient on L2.
  /// @param feeCreditData The feeCreditData struct with feeParameters snapshot from GasOracle and feeCredit captured
  /// from depositor
  event MessageSent(
    address indexed messageSender,
    address indexed messageTarget,
    uint256 indexed messageNonce,
    bytes message,
    bytes32 messageHash,
    NilConstants.MessageType messageType,
    uint256 messageCreatedAt,
    uint256 messageExpiryTime,
    address l2FeeRefundAddress,
    FeeCreditData feeCreditData
  );

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
    FeeCreditData memory feeCreditData
  ) external payable;
}
