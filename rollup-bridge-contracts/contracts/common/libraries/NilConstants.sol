// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

/// @title NilConstants
/// @notice Contains constants for bridge, messenger and rollup contracts.
library NilConstants {
  bytes32 public constant OWNER_ROLE = keccak256("OWNER_ROLE");

  bytes32 public constant PROPOSER_ROLE = keccak256("PROPOSER_ROLE");
  bytes32 public constant PROPOSER_ROLE_ADMIN = keccak256("PROPOSER_ROLE_ADMIN");

  bytes32 public constant RELAYER_ROLE_ADMIN = keccak256("RELAYER_ROLE_ADMIN");
  bytes32 public constant RELAYER_ROLE = keccak256("RELAYER_ROLE");

  /// @notice Enum representing the type of messages.
  enum MessageType {
    DEPOSIT_ERC20,
    DEPOSIT_ETH,
    WITHDRAW_ENSHRINED_TOKEN,
    WITHDRAW_ETH
  }

  /**
   * @dev Storage slot with the address of the current implementation.
   * This is the keccak-256 hash of "eip1967.proxy.implementation" subtracted by 1, and is
   * validated in the constructor.
   */
  bytes32 public constant IMPLEMENTATION_SLOT = 0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc;
}
