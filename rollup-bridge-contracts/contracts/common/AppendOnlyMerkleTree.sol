// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import { IAppendOnlyMerkleTree } from "../interfaces/IAppendOnlyMerkleTree.sol";

/**
 * @title AppendOnlyMerkleTree
 * @notice Abstract contract for maintaining an append-only Merkle tree.
 * @dev This contract allows incremental updates to the Merkle tree by appending new leaf hashes.
 *      It supports efficient computation of the Merkle root and ensures gas-efficient operations.
 *      The tree is initialized with a maximum height of 40, allowing up to 2^40 leaf hashes.
 *      This contract is designed to be inherited by other contracts that require Merkle tree functionality.
 */
abstract contract AppendOnlyMerkleTree is IAppendOnlyMerkleTree {
  /// @dev The maximum height of the Merkle tree.
  /// This defines the maximum number of levels in the Merkle tree.
  /// A height of 40 allows the tree to support up to 2^40 leaf hashes.
  /// This limit ensures that the tree remains computationally efficient and avoids excessive gas costs.
  uint256 private constant MAX_TREE_HEIGHT = 40;

  /// @notice Thrown when attempting to append a message hash before initializing the Merkle tree
  error ErrorMerkleTreeNotInitialized();

  /// @notice The merkle root of the current merkle tree.
  /// @dev This is actual equal to `branches[n]`.
  bytes32 public override messageRoot;

  /// @notice The next unused message index.
  uint256 public override nextMessageIndex;

  /// @notice The list of zero hash in each height.
  bytes32[MAX_TREE_HEIGHT] private zeroHashes;

  /// @notice The list of minimum merkle proofs needed to compute next root.
  /// @dev Only first `n` elements are used, where `n` is the minimum value that `2^{n-1} >= currentMaxNonce + 1`.
  /// It means we only use `currentMaxNonce + 1` leaf nodes to construct the merkle tree.
  bytes32[MAX_TREE_HEIGHT] public branches;

  /**
   * @notice Initializes the Merkle tree by precomputing zero hashes for all levels.
   * @dev This function must be called before appending any leaf hashes to the tree.
   *      The zero hashes are used to fill empty nodes in the sparse Merkle tree.
   */
  function _initializeMerkleTree() internal {
    // Compute hashes in empty sparse Merkle tree
    for (uint256 height = 0; height + 1 < MAX_TREE_HEIGHT; height++) {
      zeroHashes[height + 1] = _efficientHash(zeroHashes[height], zeroHashes[height]);
    }
  }

  /**
   * @dev Public function to check if the Merkle tree has been initialized.
   * @return bool True if the Merkle tree is initialized, false otherwise.
   */
  function _isMerkleTreeInitialized() internal view returns (bool) {
    // Check if the last element of zeroHashes is non-zero
    return zeroHashes[MAX_TREE_HEIGHT - 1] != bytes32(0);
  }

  /**
   * @notice Appends a new leaf hash to the Merkle tree and updates the Merkle root.
   * @dev This function computes the new Merkle root incrementally by hashing the new leaf
   *      with existing branches and zero hashes as needed.
   * @param _messageHash The hash of the message to append as a new leaf.
   * @return _currentMessageIndex The index of the newly appended leaf in the tree.
   * @return _hash The updated Merkle root after appending the new leaf.
   * @custom:throws ErrorMerkleTreeNotInitialized If the Merkle tree has not been initialized.
   */
  function _appendMessageHash(bytes32 _messageHash) internal returns (uint256 _currentMessageIndex, bytes32 _hash) {
    if (zeroHashes[1] == bytes32(0)) {
      revert ErrorMerkleTreeNotInitialized();
    }

    _currentMessageIndex = nextMessageIndex;
    _hash = _messageHash;
    uint256 _height = 0;

    while (_currentMessageIndex != 0) {
      if (_currentMessageIndex % 2 == 0) {
        branches[_height] = _hash;
        _hash = _efficientHash(_hash, zeroHashes[_height]);
      } else {
        _hash = _efficientHash(branches[_height], _hash);
      }
      unchecked {
        _height += 1;
      }
      _currentMessageIndex >>= 1;
    }

    branches[_height] = _hash;
    messageRoot = _hash;

    unchecked {
      nextMessageIndex = _currentMessageIndex + 1;
    }
  }

  /**
   * @notice Computes the keccak256 hash of two concatenated `bytes32` values.
   * @dev This function uses inline assembly for gas efficiency.
   * @param a The first `bytes32` value.
   * @param b The second `bytes32` value.
   * @return value The keccak256 hash of the concatenated values.
   */
  function _efficientHash(bytes32 a, bytes32 b) private pure returns (bytes32 value) {
    // solhint-disable-next-line no-inline-assembly
    assembly {
      mstore(0x00, a)
      mstore(0x20, b)
      value := keccak256(0x00, 0x40)
    }
  }

  /// @notice Generates a Merkle proof for a given leaf hash.
  /// @param leafIndex The index of the leaf hash in the tree.
  /// @return proof An array of sibling hashes needed to verify the inclusion of the leaf hash.
  function generateProof(uint256 leafIndex) public view returns (bytes32[] memory proof) {
    require(leafIndex < nextMessageIndex, "Leaf index out of bounds");

    uint256 proofLength = 0;
    uint256 tempIndex = nextMessageIndex;
    while (tempIndex > 1) {
      proofLength++;
      tempIndex >>= 1;
    }

    proof = new bytes32[](proofLength);
    uint256 proofIndex = 0;
    uint256 currentIndex = leafIndex;

    for (uint256 height = 0; height < proofLength; height++) {
      if (currentIndex % 2 == 0) {
        proof[proofIndex++] = branches[height];
      } else {
        proof[proofIndex++] = branches[height];
      }
      currentIndex >>= 1;
    }

    return proof;
  }

  /// @notice Verifies the inclusion of a leaf hash in the Merkle tree.
  /// @param leafHash The hash of the leaf to verify.
  /// @param proof The Merkle proof (array of sibling hashes).
  /// @return isValid True if the proof is valid and the leaf is part of the tree, false otherwise.
  function verifyInclusion(bytes32 leafHash, bytes32[] memory proof) public view returns (bool isValid) {
    bytes32 computedHash = leafHash;

    for (uint256 i = 0; i < proof.length; i++) {
      bytes32 siblingHash = proof[i];

      // Determine the order of concatenation based on the sibling hash
      if (computedHash < siblingHash) {
        // Current node is a left child
        computedHash = keccak256(abi.encodePacked(computedHash, siblingHash));
      } else {
        // Current node is a right child
        computedHash = keccak256(abi.encodePacked(siblingHash, computedHash));
      }
    }

    // Compare the computed root with the stored root
    return computedHash == messageRoot;
  }
}
