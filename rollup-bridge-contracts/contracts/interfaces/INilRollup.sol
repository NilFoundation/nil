// SPDX-License-Identifier: MIT
pragma solidity 0.8.27;

import { INilAccessControl } from "./INilAccessControl.sol";

interface INilRollup is INilAccessControl {
    /*//////////////////////////////////////////////////////////////////////////
                                       EVENTS
    //////////////////////////////////////////////////////////////////////////*/

    /// @notice Emitted when a new batch is committed.
    /// @param batchIndex The index of the batch.
    event BatchCommitted(string indexed batchIndex);

    /// @notice Emitted when a stateRoot is updated successfully
    /// @param batchIndex The index of the batch
    /// @param oldStateRoot The stateRoot of last finalized Batch which is also the prevStateRoot for current batch
    /// @param newStateRoot The stateRoot of the current BatchIndex
    event StateRootUpdated(string indexed batchIndex, bytes32 oldStateRoot, bytes32 newStateRoot);

    /*//////////////////////////////////////////////////////////////////////////
                                       STRUCTS
    //////////////////////////////////////////////////////////////////////////*/

    struct PublicDataInfo {
        /// @notice Placeholder 1
        bytes placeholder1;
        /// @notice Placeholder 2
        bytes placeholder2;
    }

    struct BatchInfo {
        /// @notice The index of the batch
        string batchIndex;
        /// @notice Whether the batch is committed
        bool isCommitted;
        /// @notice Whether the batch is finalized
        bool isFinalized;
        /// @notice The versioned hashes of the blobs in the batch
        bytes32[] versionedHashes;
        /// @notice The old state root
        bytes32 oldStateRoot;
        /// @notice The new state root
        bytes32 newStateRoot;
        /// @notice The data proofs
        bytes[] dataProofs;
        /// @notice The validity proof
        bytes validityProof;
        /// @notice The public data inputs
        PublicDataInfo publicDataInputs;
        /// @notice The number of blobs in the batch
        uint256 blobCount;
    }

    /*//////////////////////////////////////////////////////////////////////////
                                       CONSTANT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /// @return The latest finalized batch index.
    function getLastFinalizedBatchIndex() external view returns (string memory);

    /// @return The last committed batch index.
    function getLastCommittedBatchIndex() external view returns (string memory);

    /// @param batchIndex The index of the batch.
    /// @return The state root of a finalized batch.
    function finalizedStateRoots(string memory batchIndex) external view returns (bytes32);

    /// @param batchIndex The index of the batch.
    /// @return The versioned hashes of the blobs in the batch.
    function getBlobVersionedHashes(string memory batchIndex) external view returns (bytes32[] memory);

    /// @param batchIndex The index of the batch.
    /// @return Whether the batch is committed by batch index.
    function isBatchCommitted(string memory batchIndex) external view returns (bool);

    /// @param batchIndex The index of the batch.
    /// @return Whether the batch is finalized by batch index.
    function isBatchFinalized(string memory batchIndex) external view returns (bool);

    /// @param stateRoot The state root of a finalized batch.
    /// @return Whether a stateRoot is finalized
    function isRootFinalized(bytes32 stateRoot) external view returns (bool);

    /// @param stateRoot The state root of a finalized batch.
    /// @return string batch index of the stateRoot
    function batchIndexOfRoot(bytes32 stateRoot) external view returns (string memory);

    /// @dev function to check dataProof
    /// @param blobVersionedHash The blob versioned hash to check.
    /// @param dataProof The dataProof used to verify the blob versioned hash.
    function verifyDataProof(bytes32 blobVersionedHash, bytes calldata dataProof) external view;

    /// @dev generatePublicInputForValidityProofVerification
    /// @param batchIndex The index of the batch.
    /// @param publicDataInfo The struct holding all essential data points to generate the publicInput hash for proof
    /// verification
    function generatePublicInputForValidityProofVerification(
        string memory batchIndex,
        PublicDataInfo calldata publicDataInfo
    )
        external
        view
        returns (bytes memory);

    /*//////////////////////////////////////////////////////////////////////////
                                       NON-CONSTANT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /**
     * @notice Commits a new batch with the specified number of blobs.
     * @dev This function allows an account with the COMMITTER_ROLE to commit a new batch.
     * @param batchIndex The index of the batch.
     * @param blobCount The number of blobs in the batch.
     */
    function commitBatch(string memory batchIndex, uint256 blobCount) external;

    /**
     * @notice Updates the state root for a batch.
     * @dev This function allows an account with the STATE_UPDATER_ROLE to update the state root for a batch.
     * @param batchIndex The index of the batch.
     * @param oldStateRoot The old state root.
     * @param newStateRoot The new state root.
     * @param dataProofs The data proofs for the blobs in the batch.
     * @param validityProof The validity proof.
     * @param publicDataInputs The public data inputs.
     */
    function updateState(
        string memory batchIndex,
        bytes32 oldStateRoot,
        bytes32 newStateRoot,
        bytes[] calldata dataProofs,
        bytes calldata validityProof,
        PublicDataInfo calldata publicDataInputs
    )
        external;

    /**
     * @notice Pauses or unpauses the contract.
     * @dev This function allows the owner to pause or unpause the contract.
     * @param _status The pause status to update.
     */
    function setPause(bool _status) external;

    /**
     * @dev Sets the address of the NilVerifier contract.
     * @param _nilVerifierAddress The new address of the NilVerifier contract.
     */
    function setVerifierAddress(address _nilVerifierAddress) external;

    function acceptOwnership() external;
}
