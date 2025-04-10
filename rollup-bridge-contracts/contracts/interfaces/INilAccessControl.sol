// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

/// @title INilAccessControl
/// @notice An interface that lets admin and owner of the NilRollup contract to perform access management Operations
/// @dev This is the base interface for NilAccessControl. NilAccessControl inherits OZ-Enumerable-AccessControl
/// contracts from library
interface INilAccessControl {
    /*//////////////////////////////////////////////////////////////////////////
                               ACCESS-CONTROL ERRORS
    //////////////////////////////////////////////////////////////////////////*/

    error ErrorCallerIsNotAdmin();

    /*//////////////////////////////////////////////////////////////////////////
                               NON-CONSTANT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /**
     * @notice Creates a new role with the specified admin role.
     * @dev This function allows an account with the appropriate permissions to create a new role and set its admin
     * role.
     * @param role The new role to be created.
     * @param adminRole The admin role that will manage the new role.
     */
    function createNewRole(bytes32 role, bytes32 adminRole) external;

    /*//////////////////////////////////////////////////////////////////////////
                               ADMIN-MANAGEMENT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /**
     * @notice Adds an admin by granting the DEFAULT_ADMIN_ROLE to the specified account.
     * @dev This function allows the owner to grant the DEFAULT_ADMIN_ROLE to another account.
     * @param account The address to be granted the DEFAULT_ADMIN_ROLE.
     */
    function addAdmin(address account) external;

    /**
     * @notice Removes an admin by revoking the DEFAULT_ADMIN_ROLE from the specified account.
     * @dev This function allows the owner to revoke the DEFAULT_ADMIN_ROLE from another account.
     * @param account The address from which the DEFAULT_ADMIN_ROLE will be revoked.
     */
    function removeAdmin(address account) external;

    /*//////////////////////////////////////////////////////////////////////////
                               ACCESS-CONTROL FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /**
     * @notice Grants the specified role to the specified account.
     * @dev The callee grantRole function has an implicit check that only the address with ROLE_ADMIN access of this
     * role or DEFAULT_ADMIN access is allowed to grant the access to the role.
     * @param role The role to be granted.
     * @param account The address to be granted the role.
     */
    function grantAccess(bytes32 role, address account) external;

    /**
     * @notice Revokes the specified role from the specified account.
     * @dev The callee revokeRole function has an implicit check that only the address with ROLE_ADMIN access of this
     * role or DEFAULT_ADMIN access is allowed to revoke the access to the role.
     * @param role The role to be revoked.
     * @param account The address from which the role will be revoked.
     */
    function revokeAccess(bytes32 role, address account) external;

    /**
     * @notice Renounces the specified role for the calling account.
     * @dev The callee renounceRole function has an implicit check if the caller indeed has access to the role before
     * renouncing.
     * @param role The role to be renounced.
     */
    function renounceAccess(bytes32 role) external;

    /*//////////////////////////////////////////////////////////////////////////
                                 CONSTANT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /**
     * @notice Returns all addresses that have been granted the ADMIN role.
     * @dev This function checks for addresses with the ADMIN role in AccessControlEnumerableUpgradeable.
     * @return An array of addresses that have the ADMIN role.
     */
    function getAllAdmins() external view returns (address[] memory);

    /**
     * @notice Returns address that have been granted the OWNER role.
     * @dev This function checks for addresses with the OWNER role in AccessControlEnumerableUpgradeable.
     * @dev there can be only one owner with access to OWNER role.
     * @return An array of addresses that have the OWNER role.
     */
    function getOwner() external view returns (address);

    /**
     * @notice Checks if the given address is an owner.
     * @dev This function verifies if the specified address has the owner role.
     * @param ownerArg The address to check for ownership.
     * @return A boolean value indicating whether the address is an owner.
     */
    function isAnOwner(address ownerArg) external view returns (bool);

    /**
     * @notice Checks if the given address is an admin.
     * @dev This function verifies if the specified address has the admin role.
     * @param adminArg The address to check for admin role.
     * @return A boolean value indicating whether the address is an admin.
     */
    function isAnAdmin(address adminArg) external view returns (bool);
}
