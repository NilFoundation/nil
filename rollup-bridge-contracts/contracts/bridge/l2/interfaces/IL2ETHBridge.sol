// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { IL2Bridge } from "./IL2Bridge.sol";
import { IL1ETHBridge } from "../../l1/interfaces/IL1ETHBridge.sol";
import { IL2ETHBridgeVault } from "./IL2ETHBridgeVault.sol";
import { IL2BridgeMessenger } from "./IL2BridgeMessenger.sol";
import { IL2BridgeRouter } from "./IL2BridgeRouter.sol";

interface IL2ETHBridge is IL2Bridge {
  /*//////////////////////////////////////////////////////////////////////////
                             ERRORS   
    //////////////////////////////////////////////////////////////////////////*/

  error ErrorInvalidEthBridgeVault();

  error ErrorInvalidWithdrawer();

  error ErrorInvalidWithdrawalAmount();

  error ErrorInsufficientWithdrawalAmount();

  error ErrorUnAuthorizedAccess();

  error ErrorIncompleteETHDeposit();

  /*//////////////////////////////////////////////////////////////////////////
                             EVENTS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Emitted when ETH deposit is finalised on L2
  /// @param from The address of sender in L1.
  /// @param to The address of recipient in L2.
  /// @param amount The amount of ETH transferred to recipient
  event FinaliseETHDeposit(address indexed from, address to, uint256 amount);

  event L2ETHBridgeVaultSet(address indexed l2ETHBridgeVault, address indexed newL2ETHBridgeVault);

  event WithdrawnETH(bytes32 indexed messageHash, address l1WithdrawalRecipient, uint256 withdrawalAmount);

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC RESTRICTED FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  function l2ETHBridgeVault() external returns (IL2ETHBridgeVault);

  function setL2ETHBridgeVault(address l2ETHBridgeVaultAddress) external;

  /*//////////////////////////////////////////////////////////////////////////
                            PUBLIC MUTATION FUNCTIONS      
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice Complete an ETH-deposit from L1 to L2 and send fund to recipient's account in L2.
  /// @dev The function should only be called by L2ScrollMessenger.
  /// @param depositorAddress The address of account who deposits the ETH in L1.
  /// @param depositAmount The amount of the ETH to deposit.
  /// @param depositRecipient The address of recipient in L2 to receive the ETH-Token.
  /// @param feeRefundRecipient The address of fee-refund recipient in L2.

  function finaliseETHDeposit(
    address depositorAddress,
    uint256 depositAmount,
    address payable depositRecipient,
    address feeRefundRecipient
  ) external payable;

  function withdrawETH(address l1WithdrawalRecipient, uint256 withdrawalAmount) external payable;
}
