// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

interface IBridge {
  /*//////////////////////////////////////////////////////////////////////////
                             ERRORS
    //////////////////////////////////////////////////////////////////////////*/

  /// @dev Thrown when the given address is `address(0)`.
  error ErrorZeroAddress();
  error UnAuthorizedCaller();
  error InvalidMessageType();
  error ErrorInvalidRouter();
  error ErrorInvalidCounterParty();
  error ErrorInvalidMessenger();
  error ErrorCallerIsNotMessenger();
  error ErrorInvalidAddress();

  /// @dev Invalid owner address.
  error ErrorInvalidOwner();

  /// @dev Invalid default admin address.
  error ErrorInvalidDefaultAdmin();

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC CONSTANT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  function getImplementation() external view returns (address);

  /**
   * @notice Pauses or unpauses the contract.
   * @dev This function allows the owner to pause or unpause the contract.
   * @param _status The pause status to update.
   */
  function setPause(bool _status) external;

  /**
   * @notice transfers ownership to the newOwner.
   * @dev This function revokes the `OWNER_ROLE` from the current owner, calls `acceptOwnership` using
   * OwnableUpgradeable's `transferOwnership` transfer the owner rights to newOwner
   * @param newOwner The address of the new owner.
   */
  function transferOwnershipRole(address newOwner) external;
}
