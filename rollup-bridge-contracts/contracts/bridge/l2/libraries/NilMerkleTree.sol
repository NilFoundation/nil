// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

/// @title NilMerkleTree
/// @notice Library to compute Merkle roots from a list of leaves
library NilMerkleTree {
    /// @notice Computes the Merkle root from the given leaves
    /// @param leaves The list of leaves
    /// @return The computed Merkle root
    function computeMerkleRoot(bytes32[] memory leaves) internal pure returns (bytes32) {
        require(leaves.length > 0, "No leaves provided");

        while (leaves.length > 1) {
            uint256 length = leaves.length;
            uint256 newLength = (length + 1) / 2;
            bytes32[] memory newLeaves = new bytes32[](newLength);

            for (uint256 i = 0; i < length / 2; i++) {
                // TODO - replace keccak256 with precompile call from Nil.sol
                newLeaves[i] = keccak256(abi.encodePacked(leaves[2 * i], leaves[2 * i + 1]));
            }

            if (length % 2 == 1) {
                newLeaves[newLength - 1] = leaves[length - 1];
            }

            leaves = newLeaves;
        }

        return leaves[0];
    }
}
