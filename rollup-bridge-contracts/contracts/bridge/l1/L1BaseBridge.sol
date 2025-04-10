// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { OwnableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import { PausableUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import { ReentrancyGuardUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";
import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import { NilConstants } from "../../common/libraries/NilConstants.sol";
import { AddressChecker } from "../../common/libraries/AddressChecker.sol";
import { StorageUtils } from "../../common/libraries/StorageUtils.sol";
import { IL1Bridge } from "./interfaces/IL1Bridge.sol";
import { IL2Bridge } from "../l2/interfaces/IL2Bridge.sol";
import { IBridge } from "../interfaces/IBridge.sol";
import { IL1BridgeMessenger } from "./interfaces/IL1BridgeMessenger.sol";
import { IL1BridgeRouter } from "./interfaces/IL1BridgeRouter.sol";
import { INilGasPriceOracle } from "./interfaces/INilGasPriceOracle.sol";
import { NilAccessControlUpgradeable } from "../../NilAccessControlUpgradeable.sol";

abstract contract L1BaseBridge is
  OwnableUpgradeable,
  PausableUpgradeable,
  NilAccessControlUpgradeable,
  ReentrancyGuardUpgradeable,
  IL1Bridge
{
  using AddressChecker for address;
  using StorageUtils for bytes32;

  /*//////////////////////////////////////////////////////////////////////////
                             ERRORS   
    //////////////////////////////////////////////////////////////////////////*/

  /// @dev Invalid counterparty bridge address.
  error ErrorInvalidCounterpartyBridge();

  /// @dev Invalid nil gas price oracle address.
  error ErrorInvalidNilGasPriceOracle();

  error ErrorInvalidL2DepositRecipient();

  error ErrorInvalidNilGasLimit();

  /// @dev Insufficient value for fee credit.
  error ErrorInsufficientValueForFeeCredit();

  /// @dev Empty deposit.
  error ErrorEmptyDeposit();

  error ErrorOnlyRouter();

  /*//////////////////////////////////////////////////////////////////////////
                             STATE-VARIABLES   
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IL1Bridge
  address public override router;

  /// @inheritdoc IL1Bridge
  address public override counterpartyBridge;

  /// @inheritdoc IL1Bridge
  address public override messenger;

  /// @inheritdoc IL1Bridge
  address public override nilGasPriceOracle;

  /// @dev The storage slots for future usage.
  uint256[50] private __gap;

  /*//////////////////////////////////////////////////////////////////////////
                             CONSTRUCTOR  
    //////////////////////////////////////////////////////////////////////////*/
  constructor() {}

  /*//////////////////////////////////////////////////////////////////////////
                             INITIALISER  
    //////////////////////////////////////////////////////////////////////////*/

  function __L1BaseBridge_init(
    address ownerAddress,
    address adminAddress,
    address messengerAddress,
    address nilGasPriceOracleAddress
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

    _setNilGasPriceOracle(nilGasPriceOracleAddress);

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
    _setNilGasPriceOracle(nilGasPriceOracleAddress);
  }

  /*//////////////////////////////////////////////////////////////////////////
                             MODIFIERS  
    //////////////////////////////////////////////////////////////////////////*/

  modifier onlyRouter() {
    if (_msgSender() != router) {
      revert ErrorOnlyRouter();
    }
    _;
  }

  /**
   * @dev Returns the current implementation address.
   */
  function getImplementation() public view override returns (address) {
    return StorageUtils.getImplementationAddress(NilConstants.IMPLEMENTATION_SLOT);
  }

  /*//////////////////////////////////////////////////////////////////////////
                             RESTRICTED FUNCTIONS  
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IL1Bridge
  function setRouter(address routerAddress) external override onlyOwnerOrAdmin whenNotPaused {
    router = routerAddress;
  }

  function _setRouter(address routerAddress) internal {
    if (
      !routerAddress.isContract() ||
      !IERC165(IL1BridgeRouter(routerAddress).getImplementation()).supportsInterface(type(IL1BridgeRouter).interfaceId)
    ) {
      revert ErrorInvalidRouter();
    }

    emit BridgeRouterSet(router, routerAddress);
    router = routerAddress;
  }

  /// @inheritdoc IL1Bridge
  function setMessenger(address messengerAddress) external override onlyOwnerOrAdmin whenNotPaused {
    _setMessenger(messengerAddress);
  }

  function _setMessenger(address messengerAddress) internal {
    if (
      !messengerAddress.isContract() ||
      !IERC165(IL1BridgeMessenger(messengerAddress).getImplementation()).supportsInterface(
        type(IL1BridgeMessenger).interfaceId
      )
    ) {
      revert ErrorInvalidMessenger();
    }
    emit BridgeMessengerSet(messenger, messengerAddress);
    messenger = messengerAddress;
  }

  /// @inheritdoc IL1Bridge
  function setCounterpartyBridge(address counterpartyBridgeAddress) external override onlyOwnerOrAdmin whenNotPaused {
    _setCounterpartyBridge(counterpartyBridgeAddress);
  }

  function _setCounterpartyBridge(address counterpartyBridgeAddress) internal {
    if (
      !counterpartyBridgeAddress.isContract() ||
      !IERC165(IL2Bridge(counterpartyBridgeAddress).getImplementation()).supportsInterface(type(IL2Bridge).interfaceId)
    ) {
      revert ErrorInvalidNilGasPriceOracle();
    }
    emit CounterpartyBridgeSet(counterpartyBridge, counterpartyBridgeAddress);
    counterpartyBridge = counterpartyBridgeAddress;
  }

  /// @inheritdoc IL1Bridge
  function setNilGasPriceOracle(address nilGasPriceOracleAddress) external override onlyOwnerOrAdmin whenNotPaused {
    _setNilGasPriceOracle(nilGasPriceOracleAddress);
  }

  function _setNilGasPriceOracle(address nilGasPriceOracleAddress) internal {
    if (
      !nilGasPriceOracleAddress.isContract() ||
      !IERC165(INilGasPriceOracle(nilGasPriceOracleAddress).getImplementation()).supportsInterface(
        type(INilGasPriceOracle).interfaceId
      )
    ) {
      revert ErrorInvalidNilGasPriceOracle();
    }

    emit NilGasPriceOracleSet(nilGasPriceOracle, nilGasPriceOracleAddress);
    nilGasPriceOracle = nilGasPriceOracleAddress;
  }

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
}
