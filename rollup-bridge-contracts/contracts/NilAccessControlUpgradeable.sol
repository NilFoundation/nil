// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { OwnableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import { AccessControlEnumerableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/extensions/AccessControlEnumerableUpgradeable.sol";
import { NilConstants } from "./common/libraries/NilConstants.sol";
import { INilAccessControlUpgradeable } from "./interfaces/INilAccessControlUpgradeable.sol";

/// @title NilAccessControlUpgradeable
/// @notice See the documentation in {INilAccessControlUpgradeable}.
abstract contract NilAccessControlUpgradeable is
  OwnableUpgradeable,
  AccessControlEnumerableUpgradeable,
  INilAccessControlUpgradeable
{
  error ErrorCallerIsNotProposer();
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

  modifier onlyProposer() {
    if (!hasRole(NilConstants.PROPOSER_ROLE, msg.sender)) {
      revert ErrorCallerIsNotProposer();
    }
    _;
  }

  /*//////////////////////////////////////////////////////////////////////////
                           ADMIN MANAGEMENT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc INilAccessControlUpgradeable
  function addAdmin(address account) external override onlyOwner {
    grantRole(DEFAULT_ADMIN_ROLE, account);
  }

  /// @inheritdoc INilAccessControlUpgradeable
  function removeAdmin(address account) external override onlyOwner {
    revokeRole(DEFAULT_ADMIN_ROLE, account);
  }

  /*//////////////////////////////////////////////////////////////////////////
                           ROLE MANAGEMENT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc INilAccessControlUpgradeable
  function createNewRole(bytes32 role, bytes32 adminRole) external override onlyRole(DEFAULT_ADMIN_ROLE) {
    _setRoleAdmin(role, adminRole);
  }

  /*//////////////////////////////////////////////////////////////////////////
                            ACCESS-CONTROL QUERY FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc INilAccessControlUpgradeable
  function grantAccess(bytes32 role, address account) external override {
    grantRole(role, account);
  }

  //// @inheritdoc INilAccessControlUpgradeable
  function revokeAccess(bytes32 role, address account) external override {
    revokeRole(role, account);
  }

  /// @inheritdoc INilAccessControlUpgradeable
  function renounceAccess(bytes32 role) external override {
    renounceRole(role, msg.sender);
  }

  /*//////////////////////////////////////////////////////////////////////////
                            PROPOSER ADMIN FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc INilAccessControlUpgradeable
  function grantProposerAdminRole(address account) external override {
    grantRole(NilConstants.PROPOSER_ROLE_ADMIN, account);
  }

  /// @inheritdoc INilAccessControlUpgradeable
  function revokeProposerAdminRole(address account) external override {
    revokeRole(NilConstants.PROPOSER_ROLE_ADMIN, account);
  }

  /*//////////////////////////////////////////////////////////////////////////
                            PROPOSER ACCESS CONTROL FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc INilAccessControlUpgradeable
  function grantProposerAccess(address account) external override {
    grantRole(NilConstants.PROPOSER_ROLE, account);
  }

  /// @inheritdoc INilAccessControlUpgradeable
  function revokeProposerAccess(address account) external override {
    revokeRole(NilConstants.PROPOSER_ROLE, account);
  }

  /*//////////////////////////////////////////////////////////////////////////
                            ACCESS-CONTROL LISTING FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @inheritdoc INilAccessControlUpgradeable
  function getAllProposers() external view override returns (address[] memory) {
    return getRoleMembers(NilConstants.PROPOSER_ROLE);
  }

  /// @inheritdoc INilAccessControlUpgradeable
  function getAllAdmins() external view override returns (address[] memory) {
    return getRoleMembers(DEFAULT_ADMIN_ROLE);
  }

  /// @inheritdoc INilAccessControlUpgradeable
  function getOwner() public view override returns (address) {
    address[] memory owners = getRoleMembers(NilConstants.OWNER_ROLE);

    if (owners.length == 0) {
      return address(0);
    }

    return owners[0];
  }

  /// @inheritdoc INilAccessControlUpgradeable
  function isAnOwner(address ownerArg) external view override returns (bool) {
    return ownerArg == getOwner();
  }

  /// @inheritdoc INilAccessControlUpgradeable
  function isAProposer(address proposerArg) external view override returns (bool) {
    return hasRole(NilConstants.PROPOSER_ROLE, proposerArg);
  }

  /// @inheritdoc INilAccessControlUpgradeable
  function isAnAdmin(address adminArg) external view override returns (bool) {
    return hasRole(DEFAULT_ADMIN_ROLE, adminArg);
  }
}
