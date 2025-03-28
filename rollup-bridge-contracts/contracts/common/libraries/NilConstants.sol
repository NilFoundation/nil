// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

/// @title NilConstants
/// @notice Contains constants for bridge, messenger and rollup contracts.
library NilConstants {
    bytes32 public constant OWNER_ROLE = keccak256("OWNER_ROLE");

    bytes32 public constant PROPOSER_ROLE = keccak256("PROPOSER_ROLE");
    bytes32 public constant PROPOSER_ROLE_ADMIN = keccak256("PROPOSER_ROLE_ADMIN");

    bytes32 public constant GAS_PRICE_SETTER_ROLE = keccak256("GAS_PRICE_SETTER_ROLE");
    bytes32 public constant GAS_PRICE_SETTER_ROLE_ADMIN = keccak256("GAS_PRICE_SETTER_ROLE_ADMIN");

    bytes32 public constant RELAYER_ROLE_ADMIN = keccak256("RELAYER_ROLE_ADMIN");
    bytes32 public constant RELAYER_ROLE = keccak256("RELAYER_ROLE");

    /// @notice Enum representing the type of messages.
    enum MessageType {
        DEPOSIT_ERC20,
        DEPOSIT_ETH
    }
}
