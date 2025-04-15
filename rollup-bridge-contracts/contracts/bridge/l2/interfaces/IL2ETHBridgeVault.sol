// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import { IL2ETHBridge } from "./IL2ETHBridge.sol";

interface IL2ETHBridgeVault is IERC165 {
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

  event L2ETHBridgeSet(address indexed l2ETHBridge, address indexed newL2ETHBridge);

  function getImplementation() external view returns (address);

  function setL2ETHBridge(address l2EthBridgeAddress) external;

  /// @notice Transfers ETH to a recipient, only callable by the L2ETHBridge contract
  /// @param depositRecipient The address of the recipient
  /// @param depositAmount The amount of ETH to transfer
  function transferETHOnDepositFinalisation(
    address depositRecipient,
    address feeRefundRecipient,
    uint256 depositAmount
  ) external;

  function returnETHOnWithdrawal(uint256 amount) external payable;

  function ethAmountTracker() external returns (uint256);

  function l2ETHBridge() external returns (IL2ETHBridge);
}
