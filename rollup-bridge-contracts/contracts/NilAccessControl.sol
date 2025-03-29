// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import {OwnableUpgradeable} from '@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol';
import {AccessControlEnumerableUpgradeable} from '@openzeppelin/contracts-upgradeable/access/extensions/AccessControlEnumerableUpgradeable.sol';
import {INilAccessControl} from './interfaces/INilAccessControl.sol';

/// @title NilAccessControl
/// @notice See the documentation in {INilAccessControl}.
abstract contract NilAccessControl is
    OwnableUpgradeable,
    AccessControlEnumerableUpgradeable,
    INilAccessControl
{
    bytes32 public constant OWNER_ROLE = keccak256('OWNER_ROLE');
    bytes32 public constant PROPOSER_ROLE = keccak256('PROPOSER_ROLE');
    bytes32 public constant PROPOSER_ROLE_ADMIN =
        keccak256('PROPOSER_ROLE_ADMIN');

    error ErrorCallerIsNotProposer();
    error ErrorCallerIsNotAdmin();

    /*//////////////////////////////////////////////////////////////////////////
                           MODIFIERS
    //////////////////////////////////////////////////////////////////////////*/

    modifier onlyAdmin() {
        if (!(hasRole(DEFAULT_ADMIN_ROLE, msg.sender))) {
            revert ErrorCallerIsNotAdmin();
        }
        _;
    }

    modifier onlyProposer() {
        if (!hasRole(PROPOSER_ROLE, msg.sender)) {
            revert ErrorCallerIsNotProposer();
        }
        _;
    }

    /*//////////////////////////////////////////////////////////////////////////
                           ADMIN MANAGEMENT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /// @inheritdoc INilAccessControl
    function addAdmin(address account) external override onlyOwner {
        grantRole(DEFAULT_ADMIN_ROLE, account);
    }

    /// @inheritdoc INilAccessControl
    function removeAdmin(address account) external override onlyOwner {
        revokeRole(DEFAULT_ADMIN_ROLE, account);
    }

    /*//////////////////////////////////////////////////////////////////////////
                           ROLE MANAGEMENT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /// @inheritdoc INilAccessControl
    function createNewRole(
        bytes32 role,
        bytes32 adminRole
    ) external override onlyRole(DEFAULT_ADMIN_ROLE) {
        _setRoleAdmin(role, adminRole);
    }

    /*//////////////////////////////////////////////////////////////////////////
                            ACCESS-CONTROL QUERY FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /// @inheritdoc INilAccessControl
    function grantAccess(bytes32 role, address account) external override {
        grantRole(role, account);
    }

    //// @inheritdoc INilAccessControl
    function revokeAccess(bytes32 role, address account) external override {
        revokeRole(role, account);
    }

    /// @inheritdoc INilAccessControl
    function renounceAccess(bytes32 role) external override {
        renounceRole(role, msg.sender);
    }

    /*//////////////////////////////////////////////////////////////////////////
                            PROPOSER ADMIN FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /// @inheritdoc INilAccessControl
    function grantProposerAdminRole(address account) external override {
        grantRole(PROPOSER_ROLE_ADMIN, account);
    }

    /// @inheritdoc INilAccessControl
    function revokeProposerAdminRole(address account) external override {
        revokeRole(PROPOSER_ROLE_ADMIN, account);
    }

    /*//////////////////////////////////////////////////////////////////////////
                            PROPOSER ACCESS CONTROL FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /// @inheritdoc INilAccessControl
    function grantProposerAccess(address account) external override {
        grantRole(PROPOSER_ROLE, account);
    }

    /// @inheritdoc INilAccessControl
    function revokeProposerAccess(address account) external override {
        revokeRole(PROPOSER_ROLE, account);
    }

    /*//////////////////////////////////////////////////////////////////////////
                            ACCESS-CONTROL LISTING FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /// @inheritdoc INilAccessControl
    function getAllProposers()
        external
        view
        override
        returns (address[] memory)
    {
        return getRoleMembers(PROPOSER_ROLE);
    }

    /// @inheritdoc INilAccessControl
    function getAllAdmins() external view override returns (address[] memory) {
        return getRoleMembers(DEFAULT_ADMIN_ROLE);
    }

    /// @inheritdoc INilAccessControl
    function getOwner() public view override returns (address) {
        address[] memory owners = getRoleMembers(OWNER_ROLE);

        if (owners.length == 0) {
            return address(0);
        }

        return owners[0];
    }

    /// @inheritdoc INilAccessControl
    function isAnOwner(address ownerArg) external view override returns (bool) {
        return ownerArg == getOwner();
    }

    /// @inheritdoc INilAccessControl
    function isAProposer(
        address proposerArg
    ) external view override returns (bool) {
        return hasRole(PROPOSER_ROLE, proposerArg);
    }

    /// @inheritdoc INilAccessControl
    function isAnAdmin(address adminArg) external view override returns (bool) {
        return hasRole(DEFAULT_ADMIN_ROLE, adminArg);
    }
}
