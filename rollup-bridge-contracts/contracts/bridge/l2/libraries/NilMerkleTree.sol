// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

/// @title NilMerkleTree
/// @notice Library to compute, verify, and generate Merkle proofs
library NilMerkleTree {
  /// @notice Computes the Merkle root from the given leaves
  /// @param leaves The list of leaves
  /// @return The computed Merkle root
  function computeMerkleRoot(bytes32[] memory leaves) internal pure returns (bytes32) {
    require(leaves.length > 0, "No leaves provided");

    bytes32 zeroHash = bytes32(0); // Define a zero hash

    while (leaves.length > 1) {
      uint256 length = leaves.length;
      uint256 newLength = (length + 1) / 2;
      bytes32[] memory newLeaves = new bytes32[](newLength);

      for (uint256 i = 0; i < length / 2; i++) {
        // Hash pairs of leaves
        newLeaves[i] = keccak256(abi.encodePacked(leaves[2 * i], leaves[2 * i + 1]));
      }

      if (length % 2 == 1) {
        // Hash the odd leaf with zeroHash
        newLeaves[newLength - 1] = keccak256(abi.encodePacked(leaves[length - 1], zeroHash));
      }

      leaves = newLeaves;
    }

    return leaves[0];
  }

  /// @notice Verifies a Merkle proof for a given leaf and root
  /// @param proof The Merkle proof (array of sibling hashes)
  /// @param root The Merkle root to verify against
  /// @param leaf The leaf hash to verify
  /// @return True if the proof is valid, false otherwise
  function verify(bytes32[] memory proof, bytes32 root, bytes32 leaf) internal pure returns (bool) {
    bytes32 computedHash = leaf;

    for (uint256 i = 0; i < proof.length; i++) {
      // Use non-commutative hashing logic to match NilMerkleTree
      computedHash = keccak256(abi.encodePacked(computedHash, proof[i]));
    }

    return computedHash == root;
  }

  /// @notice Generates a Merkle proof for a given leaf
  /// @param leaves The list of leaves
  /// @param leaf The leaf hash for which the proof is to be generated
  /// @return proof The Merkle proof (array of sibling hashes)
  function generateProof(bytes32[] memory leaves, bytes32 leaf) internal pure returns (bytes32[] memory proof) {
    require(leaves.length > 0, "No leaves provided");

    bytes32 zeroHash = bytes32(0); // Define a zero hash
    uint256 index = findLeafIndex(leaves, leaf);
    require(index < leaves.length, "Leaf not found");

    uint256 proofLength = log2ceil(leaves.length);
    proof = new bytes32[](proofLength);

    uint256 proofIndex = 0;

    while (leaves.length > 1) {
      uint256 length = leaves.length;
      uint256 newLength = (length + 1) / 2;
      bytes32[] memory newLeaves = new bytes32[](newLength);

      for (uint256 i = 0; i < length / 2; i++) {
        // Hash pairs of leaves
        newLeaves[i] = keccak256(abi.encodePacked(leaves[2 * i], leaves[2 * i + 1]));

        // Add sibling hash to proof if the current index matches
        if (index == 2 * i) {
          proof[proofIndex++] = leaves[2 * i + 1];
        } else if (index == 2 * i + 1) {
          proof[proofIndex++] = leaves[2 * i];
        }
      }

      if (length % 2 == 1) {
        // Hash the odd leaf with zeroHash
        newLeaves[newLength - 1] = keccak256(abi.encodePacked(leaves[length - 1], zeroHash));

        // Add zeroHash to proof if the current index matches the odd leaf
        if (index == length - 1) {
          proof[proofIndex++] = zeroHash;
        }
      }

      index /= 2;
      leaves = newLeaves;
    }

    // Resize the proof array to the actual proof length
    assembly {
      mstore(proof, proofIndex)
    }

    return proof;
  }

  /// @notice Finds the index of a given leaf in the list of leaves
  /// @param leaves The list of leaves
  /// @param leaf The leaf hash to find
  /// @return The index of the leaf
  function findLeafIndex(bytes32[] memory leaves, bytes32 leaf) internal pure returns (uint256) {
    for (uint256 i = 0; i < leaves.length; i++) {
      if (leaves[i] == leaf) {
        return i;
      }
    }
    revert("Leaf not found");
  }

  /// @notice Computes the ceiling of log2 for a given number
  /// @param x The input number
  /// @return The ceiling of log2(x)
  function log2ceil(uint256 x) internal pure returns (uint256) {
    uint256 result = 0;
    uint256 value = 1;

    while (value < x) {
      value *= 2;
      result++;
    }

    return result;
  }
}
