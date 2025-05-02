// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import { IL1Bridge } from "./IL1Bridge.sol";
import { IL2EnshrinedTokenBridge } from "../../l2/interfaces/IL2EnshrinedTokenBridge.sol";
import { IRelayMessage } from "./IRelayMessage.sol";

/// @title IL1ERC20Bridge
/// @author Nil
/// @notice Interface for the L1ERC20Bridge to facilitate ERC20-Token deposits from L1 and L2
/// @notice Interface for the L1ERC20Bridge to finalize the ERC20-Token withdrawals from L2 and L1
interface IL1ERC20Bridge is IL1Bridge {
  // struct to group related variables for sendMessage
  struct DepositERC20MessageData {
    address l1Token;
    address l2Token;
    address depositorAddress;
    uint256 depositAmount;
    address l2DepositRecipient;
    address l2FeeRefundAddress;
    bytes data;
    IRelayMessage.FeeCreditData feeCreditData;
  }

  struct DepositMessageParams {
    address l1Token;
    address l2Token;
    address depositorAddress;
    uint256 depositAmount;
    address l2DepositRecipient;
    address l2FeeRefundAddress;
    uint256 l2GasLimit;
    uint256 userMaxFeePerGas;
    uint256 userMaxPriorityFeePerGas;
    bytes message;
    bytes additionalData;
    IRelayMessage.FeeCreditData feeCreditData;
  }

  /*//////////////////////////////////////////////////////////////////////////
                             ERRORS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Thrown when the token address is invalid
  error ErrorInvalidTokenAddress();

  /// @notice Thrown when the WETH token is not supported on the ERC20 bridge
  error ErrorWETHTokenNotSupported();

  /// @notice Thrown when the L2 token address is invalid
  error ErrorInvalidL2Token();

  /// @notice Thrown when the token is not supported
  error ErrorTokenNotSupported();

  /// @notice Thrown when the amount pulled by the bridge is incorrect
  error ErrorIncorrectAmountPulledByBridge();

  /// @notice Thrown when the counterparty ERC20 bridge address is invalid
  error ErrorInvalidCounterpartyERC20Bridge();

  /// @notice Thrown when the WETH token address is invalid
  error ErrorInvalidWethToken();

  /// @notice Thrown when the function selector for finalizing the deposit is invalid
  error ErrorInvalidFinaliseDepositFunctionSelector();

  /*//////////////////////////////////////////////////////////////////////////
                             EVENTS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Emitted when token mapping for ERC20 token is updated.
  /// @param l1Token The address of ERC20 token in layer-1.
  /// @param oldL2Token The address of the old ERC20Token-Address in nil-chain.
  /// @param newL2Token The address of the new ERC20Token-Address in nil-chain.
  event UpdatedTokenMapping(address indexed l1Token, address indexed oldL2Token, address indexed newL2Token);

  event DepositCancelled(
    bytes32 indexed messageHash,
    address indexed l1Token,
    address indexed cancelledDepositRecipient,
    uint256 amount
  );

  event FinalisedERC20Withdrawal(
    address l1Token,
    address l2Token,
    address l1WithdrawRecipient,
    uint256 withdrawalAmount
  );

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC CONSTANT FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Returns the L2 token address corresponding to the given L1 token address
  /// @param l1TokenAddress The address of the L1 token
  /// @return The address of the corresponding L2 token
  function getL2TokenAddress(address l1TokenAddress) external view returns (address);

  /// @notice Returns the address of the WETH token
  /// @return The address of the WETH token
  function wethToken() external view returns (address);

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC MUTATING FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  /**
   * @notice Initiates the ERC20 tokens to the nil-chain. for a specified recipient.
   * @param l1Token The address of the ERC20 in L1 token to deposit.
   * @param l2DepositRecipient The recipient address to receive the token in nil-chain.
   * @param depositAmount The amount of tokens to deposit.
   * @param l2GasLimit The gas limit required to complete the deposit on nil-chain..
   */
  function depositERC20(
    address l1Token,
    uint256 depositAmount,
    address l2DepositRecipient,
    address l2FeeRefundAddress,
    uint256 l2GasLimit,
    uint256 userMaxFeePerGas,
    uint256 userMaxPriorityFeePerGas
  ) external payable;

  function depositERC20ViaRouter(
    address l1Token,
    address depositorAddress,
    uint256 depositAmount,
    address l2DepositRecipient,
    address l2FeeRefundAddress,
    uint256 l2GasLimit,
    uint256 userMaxFeePerGas,
    uint256 userMaxPriorityFeePerGas
  ) external payable;

  function setTokenMapping(address l1TokenAddress, address l2EnshrinedTokenAddress) external;

  /// @notice finalise ERC20 withdraw from Nil to L1 and send tokens to recipient's account in L1.
  /// @dev The function should only be called by L1ScrollMessenger.
  /// @param l1Token The address of L1 token.
  /// @param l2Token The address of NilToken
  /// @param l2Withdrawer The address of account who withdraw the token in L2.
  /// @param l1WithdrawRecipient The address of recipient in L1 to receive the token.
  /// @param withdrawalAmount The amount of the token to withdraw.
  function finaliseWithdrawERC20(
    address l1Token,
    address l2Token,
    address l2Withdrawer,
    address l1WithdrawRecipient,
    uint256 withdrawalAmount
  ) external;
}
