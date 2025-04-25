// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { OwnableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import { AccessControlEnumerableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/extensions/AccessControlEnumerableUpgradeable.sol";
import { NilConstants } from "./common/libraries/NilConstants.sol";
import { IL2AccessControlUpgradeable } from "./interfaces/IL2AccessControlUpgradeable.sol";

/// @title L2AccessControlUpgradeable
/// @notice See the documentation in {IL2AccessControlUpgradeable}.
abstract contract L2AccessControlUpgradeable is
  OwnableUpgradeable,
  AccessControlEnumerableUpgradeable,
  IL2AccessControlUpgradeable
{
  error ErrorCallerIsNotAdmin();
  error ErrorCallerNotAuthorised();

  /*//////////////////////////////////////////////////////////////////////////
                           MODIFIERS
    //////////////////////////////////////////////////////////////////////////*/

  modifier onlyAdmin() {
    if (!(hasRole(DEFAULT_ADMIN_ROLE, msg.sender))) {
      revert ErrorCallerIsNotAdmin();
    }
    _;
  }

  modifier onlyOwnerOrAdmin() {
    if (!(hasRole(DEFAULT_ADMIN_ROLE, msg.sender)) && !(hasRole(NilConstants.OWNER_ROLE, msg.sender))) {
      revert ErrorCallerNotAuthorised();
    }
    _;
  }

  /*//////////////////////////////////////////////////////////////////////////
                           ADMIN MANAGEMENT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IL2AccessControlUpgradeable
  function addAdmin(address account) external override onlyOwner {
    grantRole(DEFAULT_ADMIN_ROLE, account);
  }

  /// @inheritdoc IL2AccessControlUpgradeable
  function removeAdmin(address account) external override onlyOwner {
    revokeRole(DEFAULT_ADMIN_ROLE, account);
  }

  /*//////////////////////////////////////////////////////////////////////////
                           ROLE MANAGEMENT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IL2AccessControlUpgradeable
  function createNewRole(bytes32 role, bytes32 adminRole) external override onlyRole(DEFAULT_ADMIN_ROLE) {
    _setRoleAdmin(role, adminRole);
  }

  /*//////////////////////////////////////////////////////////////////////////
                            ACCESS-CONTROL QUERY FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IL2AccessControlUpgradeable
  function grantAccess(bytes32 role, address account) external override {
    grantRole(role, account);
  }

  //// @inheritdoc IL2AccessControlUpgradeable
  function revokeAccess(bytes32 role, address account) external override {
    revokeRole(role, account);
  }

  /// @inheritdoc IL2AccessControlUpgradeable
  function renounceAccess(bytes32 role) external override {
    renounceRole(role, msg.sender);
  }

  /*//////////////////////////////////////////////////////////////////////////
                            ACCESS-CONTROL LISTING FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc IL2AccessControlUpgradeable
  function getAllAdmins() external view override returns (address[] memory) {
    return getRoleMembers(DEFAULT_ADMIN_ROLE);
  }

  /// @inheritdoc IL2AccessControlUpgradeable
  function getOwner() public view override returns (address) {
    address[] memory owners = getRoleMembers(NilConstants.OWNER_ROLE);

    if (owners.length == 0) {
      return address(0);
    }

    return owners[0];
  }

  /// @inheritdoc IL2AccessControlUpgradeable
  function isAnOwner(address ownerArg) external view override returns (bool) {
    return ownerArg == getOwner();
  }

  /// @inheritdoc IL2AccessControlUpgradeable
  function isAnAdmin(address adminArg) external view override returns (bool) {
    return hasRole(DEFAULT_ADMIN_ROLE, adminArg);
  }
}
