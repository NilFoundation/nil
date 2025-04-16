// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { OwnableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import { PausableUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import { NilAccessControlUpgradeable } from "../../NilAccessControlUpgradeable.sol";
import { ReentrancyGuardUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";
import { AccessControlEnumerableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/extensions/AccessControlEnumerableUpgradeable.sol";
import { EnumerableSet } from "@openzeppelin/contracts/utils/structs/EnumerableSet.sol";
import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import { NilConstants } from "../../common/libraries/NilConstants.sol";
import { AddressChecker } from "../../common/libraries/AddressChecker.sol";
import { IL2Bridge } from "./interfaces/IL2Bridge.sol";
import { IL1ETHBridge } from "../l1/interfaces/IL1ETHBridge.sol";
import { IL2ETHBridge } from "./interfaces/IL2ETHBridge.sol";
import { IL2ETHBridgeVault } from "./interfaces/IL2ETHBridgeVault.sol";
import { IL2BridgeMessenger } from "./interfaces/IL2BridgeMessenger.sol";
import { IL2BridgeRouter } from "./interfaces/IL2BridgeRouter.sol";
import { L2BaseBridge } from "./L2BaseBridge.sol";

contract L2ETHBridge is L2BaseBridge, IL2ETHBridge {
  using EnumerableSet for EnumerableSet.AddressSet;
  using AddressChecker for address;

  /*//////////////////////////////////////////////////////////////////////////
                             STATE-VARIABLES   
    //////////////////////////////////////////////////////////////////////////*/

  IL2ETHBridgeVault public override l2ETHBridgeVault;

  /// @dev The storage slots for future usage.
  uint256[50] private __gap;

  /*//////////////////////////////////////////////////////////////////////////
                                    CONSTRUCTOR
    //////////////////////////////////////////////////////////////////////////*/

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /*//////////////////////////////////////////////////////////////////////////
                                    INITIALIZER
    //////////////////////////////////////////////////////////////////////////*/

  function initialize(
    address ownerAddress,
    address adminAddress,
    address messengerAddress,
    address l2ETHBridgeVaultAddress
  ) public initializer {
    // Validate input parameters
    if (ownerAddress == address(0)) {
      revert ErrorInvalidOwner();
    }

    if (adminAddress == address(0)) {
      revert ErrorInvalidDefaultAdmin();
    }

    _setL2ETHBridgeVault(l2ETHBridgeVaultAddress);

    L2BaseBridge.__L2BaseBridge_init(ownerAddress, adminAddress, messengerAddress);
  }

  /*//////////////////////////////////////////////////////////////////////////
                                    PUBLIC MUTATION FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  function finaliseETHDeposit(
    address depositorAddress,
    uint256 depositAmount,
    address payable depositRecipient,
    address feeRefundRecipient
  ) public payable override onlyMessenger whenNotPaused {
    // get recipient balance before ETH transfer
    uint256 befBalance = depositRecipient.balance;

    // call sendEth on L2ETHBridgeVault
    l2ETHBridgeVault.transferETHOnDepositFinalisation(depositRecipient, feeRefundRecipient, depositAmount);

    // emit FinalisedETHDepositEvent
    emit FinaliseETHDeposit(depositorAddress, depositRecipient, depositAmount);
  }

  function withdrawETH(address l1WithdrawalRecipient, uint256 withdrawalAmount) public payable override whenNotPaused {
    if (!l1WithdrawalRecipient.isContract()) {
      revert ErrorInvalidAddress();
    }

    if (withdrawalAmount == 0) {
      revert ErrorInvalidWithdrawalAmount();
    }

    if (msg.value != withdrawalAmount) {
      revert ErrorInsufficientWithdrawalAmount();
    }

    // return the ETH to the ETHBridgeVault
    l2ETHBridgeVault.returnETHOnWithdrawal{ value: withdrawalAmount }(withdrawalAmount);

    // Generate message to be executed on L1ETHBridge
    bytes memory message = abi.encodeCall(
      IL1ETHBridge.finaliseETHWithdrawal,
      (l1WithdrawalRecipient, withdrawalAmount)
    );

    // Send message to L2BridgeMessenger.
    bytes32 messageHash = IL2BridgeMessenger(messenger).sendMessage(
      NilConstants.MessageType.WITHDRAW_ETH,
      counterpartyBridge,
      message
    );

    if (messageHash == bytes32(0)) {
      revert ErrorInvalidMessageHash();
    }

    emit WithdrawnETH(messageHash, l1WithdrawalRecipient, withdrawalAmount);
  }

  /*//////////////////////////////////////////////////////////////////////////
                         RESTRICTED FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IL2ETHBridge
  function setL2ETHBridgeVault(address l2ETHBridgeVaultAddress) external override onlyOwnerOrAdmin {
    _setL2ETHBridgeVault(l2ETHBridgeVaultAddress);
  }

  function _setL2ETHBridgeVault(address l2ETHBridgeVaultAddress) internal {
    if (
      !l2ETHBridgeVaultAddress.isContract() ||
      !IERC165(IL2ETHBridgeVault(l2ETHBridgeVaultAddress).getImplementation()).supportsInterface(
        type(IL2ETHBridgeVault).interfaceId
      )
    ) {
      revert ErrorInvalidEthBridgeVault();
    }

    emit L2ETHBridgeVaultSet(address(l2ETHBridgeVault), l2ETHBridgeVaultAddress);
    l2ETHBridgeVault = IL2ETHBridgeVault(l2ETHBridgeVaultAddress);
  }

  /// @inheritdoc IERC165
  function supportsInterface(bytes4 interfaceId) public view override returns (bool) {
    return
      interfaceId == type(IL2ETHBridge).interfaceId ||
      interfaceId == type(IL2Bridge).interfaceId ||
      super.supportsInterface(interfaceId);
  }
}
