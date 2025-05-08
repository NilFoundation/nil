// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import { IL2ETHBridge } from "./IL2ETHBridge.sol";

interface IL2ETHBridgeVault is IERC165 {
  /*//////////////////////////////////////////////////////////////////////////
                             ERRORS
    //////////////////////////////////////////////////////////////////////////*/

  error ErrorInvalidL2ETHBridge();
  error ErrorCallerNotL2ETHBridge();
  error ErrorInvalidRecipientAddress();
  error ErrorInvalidTransferAmount();
  error ErrorInsufficientVaultBalance();
  error ErrorUnauthorisedFunding();
  /// @dev Invalid owner address.
  error ErrorInvalidOwner();

  error ErrorETHTransferFailed();

  /// @dev Invalid default admin address.
  error ErrorInvalidDefaultAdmin();

  /// @dev Invalid address.
  error ErrorInvalidAddress();

  error ErrorInvalidReturnAmount();

  error ErrorInsufficientReturnAmount();

  error ErrorETHReturnedOnWithdrawalFailed();

  error ErrorCallerIsNotAdmin();
  error ErrorCallerNotAuthorised();

  /*//////////////////////////////////////////////////////////////////////////
                             EVENTS
    //////////////////////////////////////////////////////////////////////////*/

  event L2ETHBridgeSet(address indexed l2ETHBridge, address indexed newL2ETHBridge);

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC RESTRICTED FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Transfers ETH to a recipient, only callable by the L2ETHBridge contract
  /// @param depositRecipient The address of the recipient
  /// @param depositAmount The amount of ETH to transfer
  function transferETHOnDepositFinalisation(
    address depositRecipient,
    address feeRefundRecipient,
    uint256 depositAmount
  ) external;

  function returnETHOnWithdrawal(uint256 amount) external payable;

  function setL2ETHBridge(address l2EthBridgeAddress) external;

  function setPause(bool _status) external;

  /*//////////////////////////////////////////////////////////////////////////
                             ACCESS CONTROL MANAGEMENT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  function grantAccess(bytes32 role, address account) external;

  function revokeAccess(bytes32 role, address account) external;

  function renounceAccess(bytes32 role) external;

  function transferOwnershipRole(address newOwner) external;

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC CONSTANT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  function getImplementation() external view returns (address);

  function ethAmountTracker() external returns (uint256);

  function l2ETHBridge() external returns (IL2ETHBridge);
}
