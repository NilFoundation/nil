// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { INilAccessControlUpgradeable } from "./INilAccessControlUpgradeable.sol";

interface INilRollup is INilAccessControlUpgradeable {
    /*//////////////////////////////////////////////////////////////////////////
                             NILROLLUP-ERRORS   
    //////////////////////////////////////////////////////////////////////////*/

    /// @dev Invalid owner address.
    error ErrorInvalidOwner();

    /// @dev Invalid address.
    error ErrorInvalidAddress();

    /// @dev Invalid default admin address.
    error ErrorInvalidDefaultAdmin();

    /// @dev Invalid chain ID.
    error ErrorInvalidChainID();

    /// @dev Invalid NilVerifier address.
    error ErrorInvalidNilVerifier();

    /// @dev Error thrown when setVerifierAddress is called with idential address as in nilVerifier
    error ErrorNilVerifierAddressNotChanged();

    /// @dev New state root is invalid.
    error ErrorInvalidNewStateRoot();

    /// @dev BatchIndex is invalid.
    error ErrorInvalidBatchIndex();

    /// @dev Old state root is invalid.
    error ErrorInvalidOldStateRoot();

    /// @dev Error when commitBatch is called on batchIndex which is already committed
    error ErrorBatchAlreadyCommitted(string batchIndex);

    /// @dev Error when commitBatch is called on batchIndex which is already finalized
    error ErrorBatchAlreadyFinalized(string batchIndex);

    /// @dev Error when the versionHash for a blob at blobIndex in invalid
    error ErrorInvalidVersionedHash(string batchIndex, uint256 blobIndex);

    /// @dev Call of kzg evaluation precompile failed for unknown reason.
    error ErrorCallEvaluationPrecompileFailed();

    /// @dev Output from evaluation precompile doesn't match expected result.
    error ErrorEvaluationPrecompileOutputWrong();

    /// @dev The current state root doesn't match the submitted old root.
    error ErrorOldStateRootMismatch();

    /// @dev The dataProof size doesn't match with the blob count of the committed batch
    error ErrorIncorrectDataProofSize();

    /// @dev New state root was already finalized.
    error ErrorNewStateRootAlreadyFinalized(string batchIndex, bytes32 newStateRoot);

    /// @dev Data proof array is invalid.
    error ErrorEmptyDataProofs();

    /// @dev Data proof array size mismatch with the blobCount
    error ErrorDataProofsAndBlobCountMismatch(uint256 dataProofCount, uint256 committedBlobCount);

    /// @dev Data proof entry is invalid.
    error ErrorInvalidDataProofItem(uint256 proofIndex);

    /// @dev publicInput for validityProof verification is invalid
    error ErrorInvalidPublicInputForProof();

    /// @dev Validity proof is invalid.
    error ErrorInvalidValidityProof();

    /// @dev Batch is not committed
    error ErrorBatchNotCommitted(string batchIndex);

    /// @dev Thrown when call precompile failed.
    error ErrorCallPointEvaluationPrecompileFailed();

    /// @dev Thrown when the precompile output is incorrect.
    error ErrorUnexpectedPointEvaluationPrecompileOutput();

    error Unauthorized(address caller);

    error ErrorInvalidL2ToL1Root();

    error ErrorDuplicateL2ToL1Root();

    error ErrorL1MessageHashMismatch(bytes32 computedL1MessageHash, bytes32 expectedL1MessageHash);

    error ErrorInvalidPublicDataInfo();

    /// @dev State root being used for state reset is invalid.
    error ErrorInvalidResetStateRoot();

    /// @dev State root being used for state reset was not found in state roots storage.
    error ErrorResetStateRootNotFound();

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

    /// @notice Emitted when the state is successfully reset to a provided state root.
    /// @param stateRoot The state root to which the system was reset.
    event StateReset(bytes32 stateRoot);

    /*//////////////////////////////////////////////////////////////////////////
                                       STRUCTS
    //////////////////////////////////////////////////////////////////////////*/

    struct PublicDataInfo {
        /// @notice The Merkle root representing the rootHash of the
        /// merkle tree which has messageHash values of failed
        /// deposits
        bytes32 l2Tol1Root;
        /// @notice number of depositMessages for verification
        uint256 messageCount;
        /// @notice l1MessageHash generated by relayer for the depositMessages
        bytes32 l1MessageHash;
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
        PublicDataInfo publicDataInfo;
        /// @notice The number of blobs in the batch
        uint256 blobCount;
    }

    /*//////////////////////////////////////////////////////////////////////////
                                       CONSTANT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /// @return The latest finalized batch index.
    function getLastFinalizedBatchIndex() external view returns (string memory);

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

    /**
     * @notice Returns the state-root from the lastFinalized Batch of the rollup.
     * @return The state-root of type bytes32 from the lastFinalized Batch
     */
    function getCurrentStateRoot() external view returns (bytes32);

    /**
     * @notice Returns the l2Tol1Root from the lastFinalized Batch of the rollup.
     * @return The l2Tol1Root of type bytes32 from the lastFinalized Batch
     */
    function getCurrentL2ToL1Root() external view returns (bytes32);

    /// @param stateRoot The state root of a finalized batch.
    /// @return string batch index of the stateRoot
    function batchIndexOfRoot(
        bytes32 stateRoot
    ) external view returns (string memory);

    /// @notice Returns the previous state root in the finalized batch history.
    /// @param stateRoot The state root of a finalized batch.
    /// @return The state root that immediately precedes the given stateRoot in the history.
    function previousStateRoot(
        bytes32 stateRoot
    ) external view returns (bytes32);

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
     * @dev This function allows an account with the PROPOSER_ROLE to update the state root for a batch.
     * @param batchIndex The index of the batch.
     * @param oldStateRoot The old state root.
     * @param newStateRoot The new state root.
     * @param dataProofs The data proofs for the blobs in the batch.
     * @param validityProof The validity proof.
     * @param publicDataInfo The public data inputs.
     */
    function updateState(
        string memory batchIndex,
        bytes32 oldStateRoot,
        bytes32 newStateRoot,
        bytes[] calldata dataProofs,
        bytes calldata validityProof,
        PublicDataInfo calldata publicDataInfo
    )
        external;

    /**
     * @dev Sets the address of the NilVerifier contract.
     * @param _nilVerifierAddress The new address of the NilVerifier contract.
     */
    function setVerifierAddress(address _nilVerifierAddress) external;

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
