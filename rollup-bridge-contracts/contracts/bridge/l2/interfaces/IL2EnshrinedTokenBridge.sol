// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { IL2Bridge } from "./IL2Bridge.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";
import "@nilfoundation/smart-contracts/contracts/Nil.sol";

interface IL2EnshrinedTokenBridge is IL2Bridge {
  /*//////////////////////////////////////////////////////////////////////////
                             ERRORS   
    //////////////////////////////////////////////////////////////////////////*/

  /*//////////////////////////////////////////////////////////////////////////
                             EVENTS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Emitted when the token mapping is updated
  /// @param l2EnshrinedTokenAddress The address of the enshrined token on L2
  /// @param l1TokenAddress The address of the corresponding token on L1
  event TokenMappingUpdated(address indexed l2EnshrinedTokenAddress, address indexed l1TokenAddress);

  /// @notice Emitted when ERC20 token is deposited from L1 to L2 and transfer to recipient.
  /// @param l1Token The address of the token in L1.
  /// @param l2Token The address of the token in L2.
  /// @param depositor The address of sender in L1.
  /// @param depositAmount The amount of token withdrawn from L1 to L2.
  /// @param depositRecipient The address of recipient in L2.
  /// @param feeRefundRecipient The address of recipient for fee-refund on L2.
  /// @param data The optional calldata passed to recipient in L2.
  event FinalizeDepositERC20(
    TokenId indexed l1Token,
    TokenId indexed l2Token,
    address indexed depositor,
    uint256 depositAmount,
    address depositRecipient,
    address feeRefundRecipient,
    bytes data
  );

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC CONSTANT FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Return the corresponding l1 token address given l2 token address.
  /// @param l2Token The address of l2 token.
  function getL1ERC20Address(address l2Token) external view returns (address);

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC MUTATION FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Complete a deposit from L1 to L2 and send fund to recipient's account in L2.
  /// @dev Make this function payable to handle WETH deposit/withdraw.
  ///      The function should only be called by L2ScrollMessenger.
  ///      The function should also only be called by L1ERC20Gateway in L1.
  /// @param l1Token The address of corresponding L1 token.
  /// @param l2Token The address of corresponding L2 token.
  /// @param depositor The address of account who deposits the token in L1.
  /// @param depositAmount The amount of the token to deposit.
  /// @param depositRecipient The address of recipient in L2 to receive the token.
  /// @param feeRefundRecipient The address of fee-refund recipient in L2.
  /// @param additionalData Optional data to hold token-metadata
  function finaliseERC20Deposit(
    address l1Token,
    address l2Token,
    address depositor,
    uint256 depositAmount,
    address depositRecipient,
    address feeRefundRecipient,
    bytes calldata additionalData
  ) external payable;

  /// @notice Sets the token mapping between L2 enshrined token and L1 token
  /// @param l2EnshrinedTokenAddress The address of the enshrined token on L2
  /// @param l1TokenAddress The address of the corresponding token on L1
  function setTokenMapping(address l2EnshrinedTokenAddress, address l1TokenAddress) external;
}
