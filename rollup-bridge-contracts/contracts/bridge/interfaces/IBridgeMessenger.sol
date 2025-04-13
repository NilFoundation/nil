// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";

interface IBridgeMessenger is IERC165 {
  /*//////////////////////////////////////////////////////////////////////////
                           ERRORS
    //////////////////////////////////////////////////////////////////////////*/

  /// @dev Thrown when the given address is `address(0)`.
  error ErrorZeroAddress();

  /// @dev Thrown when the given hash is invalid.
  error ErrorInvalidHash();

  /// @dev Thrown when the given amount is invalid.
  error ErrorInvalidAmount();

  /// @dev Thrown when the given gas limit is invalid.
  error ErrorInvalidGasLimit();

  /// @dev Invalid owner address.
  error ErrorInvalidOwner();

  /// @dev Invalid default admin address.
  error ErrorInvalidDefaultAdmin();

  error ErrorInvalidAddress();

  error ErrorBridgeNotAuthorised();

  error ErrorRelayerNotAuthorised();

  /// @notice Thrown when a bridge interface is invalid.
  error ErrorInvalidBridgeInterface();

  /// @notice Thrown when a bridge is already authorized.
  error ErrorBridgeAlreadyAuthorised();

  error ErrorInvalidCounterpartBridgeMessenger();

  error ErrorDuplicateMessageRelayed(bytes32 messageHash);

  error ErrorInvalidMerkleRoot();

  error ErrorInvalidBridgeMessenger();

  event CounterpartyBridgeMessengerSet(
    address indexed counterpartyBridgeMessenger,
    address indexed counterpartyBridgeMessengerAddress
  );

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

  function getImplementation() external view returns (address);
}
