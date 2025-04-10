// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";
import { AccessControlEnumerable } from "@openzeppelin/contracts/access/extensions/AccessControlEnumerable.sol";
import { NilConstants } from "./common/libraries/NilConstants.sol";
import { INilAccessControl } from "./interfaces/INilAccessControl.sol";

/// @title NilAccessControl
/// @notice See the documentation in {INilAccessControl}.
abstract contract NilAccessControl is Ownable, AccessControlEnumerable, INilAccessControl {
    /*//////////////////////////////////////////////////////////////////////////
                           MODIFIERS
    //////////////////////////////////////////////////////////////////////////*/

    modifier onlyAdmin() {
        if (!(hasRole(DEFAULT_ADMIN_ROLE, msg.sender))) {
            revert ErrorCallerIsNotAdmin();
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
    function createNewRole(bytes32 role, bytes32 adminRole) external override onlyRole(DEFAULT_ADMIN_ROLE) {
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
                            ACCESS-CONTROL LISTING FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /// @inheritdoc INilAccessControl
    function getAllAdmins() external view override returns (address[] memory) {
        return getRoleMembers(DEFAULT_ADMIN_ROLE);
    }

    /// @inheritdoc INilAccessControl
    function getOwner() public view override returns (address) {
        address[] memory owners = getRoleMembers(NilConstants.OWNER_ROLE);

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
    function isAnAdmin(address adminArg) external view override returns (bool) {
        return hasRole(DEFAULT_ADMIN_ROLE, adminArg);
    }
}
