// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { IL1Bridge } from "./IL1Bridge.sol";
import { IL2EnshrinedTokenBridge } from "../../l2/interfaces/IL2EnshrinedTokenBridge.sol";

/// @title IL1ERC20Bridge
/// @author Nil
/// @notice Interface for the L1ERC20Bridge to facilitate ERC20-Token deposits from L1 and L2
/// @notice Interface for the L1ERC20Bridge to finalize the ERC20-Token withdrawals from L2 and L1
interface IL1ERC20Bridge is IL1Bridge {
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
  event UpdateTokenMapping(address indexed l1Token, address indexed oldL2Token, address indexed newL2Token);

  /// @notice Emitted upon deposit of ERC20Token from layer-1 to nil-chain.
  /// @param l1Token The address of the token in layer-1.
  /// @param l2Token The address of the token in nil-chain.
  /// @param depositor The address of sender in layer-1.
  /// @param l2Recipient The address of recipient in nil-chain.
  /// @param amount The amount of token will be deposited from layer-1 to nil-chain.
  /// @param data The optional calldata passed to recipient in nil-chain.
  event DepositERC20(
    address indexed l1Token,
    address indexed l2Token,
    address indexed depositor,
    address l2Recipient,
    uint256 amount,
    bytes data
  );

  event DepositCancelled(
    bytes32 indexed messageHash,
    address indexed l1Token,
    address indexed cancelledDepositRecipient,
    uint256 amount
  );

  /*//////////////////////////////////////////////////////////////////////////
                             STRUCTS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Represents the decoded data of an ERC20 deposit message
  struct ERC20DepositMessage {
    address l1Token;
    address l2Token;
    address depositorAddress;
    address l2DepositRecipient;
    /// @notice The address for fees refund - l2FeesComponent for deposit
    address l2FeeRefundRecipient;
    /// @notice The amount of tokens to deposit
    uint256 depositAmount;
    /// @notice Additional data for the recipient
    bytes additionalData;
  }

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

  /// @notice Decodes an encoded ERC20 deposit message
  /// @param message The encoded message to decode
  /// @return A struct containing the decoded message data
  function decodeERC20DepositMessage(bytes memory message) external pure returns (ERC20DepositMessage memory);

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC MUTATING FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  /**
   * @notice Initiates the ERC20 tokens to the nil-chain. for a specified recipient.
   * @param l1Token The address of the ERC20 in L1 token to deposit.
   * @param l2Recipient The recipient address to receive the token in nil-chain.
   * @param amount The amount of tokens to deposit.
   * @param l2FeeRefundRecipient The recipient address to receive the refund of excess fee on nil-chain
   * @param gasLimit The gas limit required to complete the deposit on nil-chain..
   */
  function depositERC20(
    address l1Token,
    address l2Recipient,
    uint256 amount,
    address l2FeeRefundRecipient,
    uint256 gasLimit,
    uint256 userFeePerGas,
    uint256 userMaxFeePerGas
  ) external payable;

  function depositERC20ViaRouter(
    address token,
    address l2DepositRecipient,
    uint256 depositAmount,
    address l2FeeRefundRecipient,
    address depositorAddress,
    uint256 l2GasLimit,
    uint256 userFeePerGas,
    uint256 userMaxPriorityFeePerGas
  ) external payable;
}
