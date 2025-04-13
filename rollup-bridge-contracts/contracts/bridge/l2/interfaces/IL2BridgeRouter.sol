// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

interface IL2BridgeRouter {
  function getImplementation() external view returns (address);

  /**
   * @notice Pauses or unpauses the contract.
   * @dev This function allows the owner to pause or unpause the contract.
   * @param statusValue The pause status to update.
   */
  function setPause(bool statusValue) external;

  /**
   * @notice transfers ownership to the newOwner.
   * @dev This function revokes the `OWNER_ROLE` from the current owner, calls `acceptOwnership` using
   * OwnableUpgradeable's `transferOwnership` transfer the owner rights to newOwner
   * @param newOwner The address of the new owner.
   */
  function transferOwnershipRole(address newOwner) external;

  // withdrawEnshrinedToken
  // withdrawETH
}
