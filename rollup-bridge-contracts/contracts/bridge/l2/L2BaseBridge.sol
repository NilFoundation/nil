// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { OwnableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import { PausableUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import { ReentrancyGuardUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";
import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import { IL1Bridge } from "../l1/interfaces/IL1Bridge.sol";
import { IL2Bridge } from "./interfaces/IL2Bridge.sol";
import { IBridge } from "../interfaces/IBridge.sol";
import { IL2BridgeMessenger } from "./interfaces/IL2BridgeMessenger.sol";
import { IL2BridgeRouter } from "./interfaces/IL2BridgeRouter.sol";
import { IBridgeMessenger } from "../interfaces/IBridgeMessenger.sol";
import { NilAccessControlUpgradeable } from "../../NilAccessControlUpgradeable.sol";
import { NilConstants } from "../../common/libraries/NilConstants.sol";
import { AddressChecker } from "../../common/libraries/AddressChecker.sol";
import { StorageUtils } from "../../common/libraries/StorageUtils.sol";

abstract contract L2BaseBridge is
  OwnableUpgradeable,
  PausableUpgradeable,
  NilAccessControlUpgradeable,
  ReentrancyGuardUpgradeable,
  IL2Bridge
{
  using AddressChecker for address;
  using StorageUtils for bytes32;

  /*//////////////////////////////////////////////////////////////////////////
                             STATE-VARIABLES   
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IL2Bridge
  address public override router;

  /// @inheritdoc IL2Bridge
  address public override counterpartyBridge;

  /// @inheritdoc IL2Bridge
  address public override messenger;

  /// @dev The storage slots for future usage.
  uint256[50] private __gap;

  /*//////////////////////////////////////////////////////////////////////////
                             CONSTRUCTOR  
    //////////////////////////////////////////////////////////////////////////*/
  constructor() {}

  /*//////////////////////////////////////////////////////////////////////////
                             INITIALISER  
    //////////////////////////////////////////////////////////////////////////*/

  function __L2BaseBridge_init(
    address ownerAddress,
    address adminAddress,
    address messengerAddress
  ) internal onlyInitializing {
    // Validate input parameters
    if (ownerAddress == address(0)) {
      revert ErrorInvalidOwner();
    }

    if (adminAddress == address(0)) {
      revert ErrorInvalidDefaultAdmin();
    }

    // Initialize the Ownable contract with the owner address
    OwnableUpgradeable.__Ownable_init(ownerAddress);

    // Initialize the Pausable contract
    PausableUpgradeable.__Pausable_init();

    // Initialize the AccessControlEnumerable contract
    __AccessControlEnumerable_init();

    // Set role admins
    // The OWNER_ROLE is set as its own admin to ensure that only the current owner can manage this role.
    _setRoleAdmin(NilConstants.OWNER_ROLE, NilConstants.OWNER_ROLE);

    // The DEFAULT_ADMIN_ROLE is set as its own admin to ensure that only the current default admin can manage this
    // role.
    _setRoleAdmin(DEFAULT_ADMIN_ROLE, NilConstants.OWNER_ROLE);

    // Grant roles to defaultAdmin and owner
    // The DEFAULT_ADMIN_ROLE is granted to both the default admin and the owner to ensure that both have the
    // highest level of control.
    // The PROPOSER_ROLE_ADMIN is granted to both the default admin and the owner to allow them to manage proposers.
    // The OWNER_ROLE is granted to the owner to ensure they have the highest level of control over the contract.
    _grantRole(NilConstants.OWNER_ROLE, ownerAddress);
    _grantRole(DEFAULT_ADMIN_ROLE, adminAddress);

    ReentrancyGuardUpgradeable.__ReentrancyGuard_init();

    _setMessenger(messengerAddress);
  }

  /*//////////////////////////////////////////////////////////////////////////
                                    MODIFIERS
    //////////////////////////////////////////////////////////////////////////*/

  modifier onlyMessenger() {
    // check caller is l2-bridge-messenger
    if (msg.sender != address(messenger)) {
      revert ErrorCallerIsNotMessenger();
    }
    _;
  }

  /*//////////////////////////////////////////////////////////////////////////
                                    RESTRICTED FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IL2Bridge
  function setRouter(address routerAddress) external override onlyOwnerOrAdmin {
    router = routerAddress;
  }

  function _setRouter(address routerAddress) internal {
    if (
      routerAddress.isContract() ||
      !IERC165(IL2BridgeRouter(routerAddress).getImplementation()).supportsInterface(type(IL2BridgeRouter).interfaceId)
    ) {
      revert ErrorInvalidRouter();
    }
    emit BridgeRouterSet(router, routerAddress);
    router = routerAddress;
  }

  /// @inheritdoc IL2Bridge
  function setMessenger(address messengerAddress) external override onlyOwnerOrAdmin {
    _setMessenger(messengerAddress);
  }

  function _setMessenger(address messengerAddress) internal {
    if (
      !messengerAddress.isContract() ||
      !IERC165(IBridgeMessenger(messengerAddress).getImplementation()).supportsInterface(
        type(IL2BridgeMessenger).interfaceId
      )
    ) {
      revert ErrorInvalidMessenger();
    }
    emit BridgeMessengerSet(messenger, messengerAddress);
    messenger = messengerAddress;
  }

  /// @inheritdoc IL2Bridge
  function setCounterpartyBridge(address counterpartyBridgeAddress) external override onlyOwnerOrAdmin {
    _setCounterpartyBridge(counterpartyBridgeAddress);
  }

  function _setCounterpartyBridge(address counterpartyBridgeAddress) internal {
    if (
      !counterpartyBridgeAddress.isContract() ||
      !IERC165(IBridge(counterpartyBridgeAddress).getImplementation()).supportsInterface(type(IL1Bridge).interfaceId)
    ) {
      revert ErrorInvalidCounterpartyBridge();
    }

    emit CounterpartyBridgeSet(counterpartyBridge, counterpartyBridgeAddress);
    counterpartyBridge = counterpartyBridgeAddress;
  }

  /*//////////////////////////////////////////////////////////////////////////
                             RESTRICTED FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IBridge
  function setPause(bool _status) external onlyOwnerOrAdmin {
    if (_status) {
      _pause();
    } else {
      _unpause();
    }
  }

  /// @inheritdoc IBridge
  function transferOwnershipRole(address newOwner) external override onlyOwner {
    _revokeRole(NilConstants.OWNER_ROLE, owner());
    super.transferOwnership(newOwner);
    _grantRole(NilConstants.OWNER_ROLE, newOwner);
  }

  /**
   * @dev Returns the current implementation address.
   */
  function getImplementation() public view override returns (address) {
    return StorageUtils.getImplementationAddress(NilConstants.IMPLEMENTATION_SLOT);
  }
}
