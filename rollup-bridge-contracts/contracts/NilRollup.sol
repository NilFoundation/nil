// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { OwnableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import { PausableUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import { NilAccessControlUpgradeable } from "./NilAccessControlUpgradeable.sol";
import { NilConstants } from "./common/libraries/NilConstants.sol";
import { INilRollup } from "./interfaces/INilRollup.sol";
import { INilVerifier } from "./interfaces/INilVerifier.sol";
import { IL1BridgeMessenger } from "./bridge/l1/interfaces/IL1BridgeMessenger.sol";

/// @title NilRollup
/// @notice Manages rollup batches, state updates, and access control for the Nil protocol.
/// @notice See the documentation in {INilAccessControlUpgradeable}.
contract NilRollup is OwnableUpgradeable, PausableUpgradeable, NilAccessControlUpgradeable, INilRollup {
    /*//////////////////////////////////////////////////////////////////////////
                                  CONSTANTS
    //////////////////////////////////////////////////////////////////////////*/
    /// @dev BLS Modulus defined in EIP-4844.
    uint256 internal constant BLS_MODULUS =
        52_435_875_175_126_190_479_447_740_508_185_965_837_690_552_500_527_637_822_603_658_699_938_581_184_513;

    /// @dev The old state root used for the genesis batch.
    bytes32 public constant ZERO_STATE_ROOT = bytes32(0);

    /// @dev The initial batch index used for the genesis batch.
    string public constant GENESIS_BATCH_INDEX = "GENESIS_BATCH_INDEX";

    /// @dev Address of the kzg precompiled contract.
    address public constant POINT_EVALUATION_PRECOMPILE_ADDR = address(0x0A);

    /*//////////////////////////////////////////////////////////////////////////
                                  STATE VARIABLES
    //////////////////////////////////////////////////////////////////////////*/

    /// @dev Layer2ChainId, Set in the constructor.
    uint64 public l2ChainId;

    /// The address of NilVerifier.
    address public nilVerifierAddress;

    /// The latest finalized batch index.
    string public lastFinalizedBatchIndex;

    /// @dev Finalized state id.
    mapping(bytes32 => string) public stateRootIndex;

    /// @dev mapping of batchIndex to BatchInformation
    mapping(string => BatchInfo) public batchInfoRecords;

    bytes32 public l1MessageHash;

    IL1BridgeMessenger public l1BridgeMessenger;

    /// @dev The storage slots for future usage.
    uint256[50] private __gap;

    /*//////////////////////////////////////////////////////////////////////////
                                    CONSTRUCTOR
    //////////////////////////////////////////////////////////////////////////*/

    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor() {
        _disableInitializers();
    }

    /*//////////////////////////////////////////////////////////////////////////
                                    INITIALIZER
    //////////////////////////////////////////////////////////////////////////*/

    /**
     * @notice Initializes the NilRollup contract.
     * @dev This function sets up the initial roles, state, and configuration for the NilRollup contract.
     * @dev deployer of contract or initializer need not be same as the owner of the contract.
     * @param _l2ChainId The chainId of the L2 (NilChain)
     * @param _owner The address of the contract owner.
     * @param _defaultAdmin The address of the default admin.
     * @param _nilVerifierAddress The address of the NilVerifier contract.
     * @param _proposer The address to be granted the PROPOSER_ROLE.
     * @param _genesisStateRoot newStateRootHash to be set for the genesisBatch in BatchInfo mapping
     */
    function initialize(
        uint64 _l2ChainId,
        address _owner,
        address _defaultAdmin,
        address _nilVerifierAddress,
        address _proposer,
        bytes32 _genesisStateRoot
    )
        public
        initializer
    {
        // Validate input parameters
        if (_owner == address(0)) {
            revert ErrorInvalidOwner();
        }

        if (_defaultAdmin == address(0)) {
            revert ErrorInvalidDefaultAdmin();
        }

        if (_nilVerifierAddress == address(0)) {
            revert ErrorInvalidNilVerifier();
        }

        // Initialize the Ownable contract with the owner address
        OwnableUpgradeable.__Ownable_init(_owner);

        // Initialize the Pausable contract
        PausableUpgradeable.__Pausable_init();

        // Initialize the AccessControlEnumerable contract
        __AccessControlEnumerable_init();

        l2ChainId = _l2ChainId;

        // Set role admins
        // The OWNER_ROLE is set as its own admin to ensure that only the current owner can manage this role.
        _setRoleAdmin(NilConstants.OWNER_ROLE, NilConstants.OWNER_ROLE);

        // The DEFAULT_ADMIN_ROLE is set as its own admin to ensure that only the current default admin can manage this
        // role.
        _setRoleAdmin(DEFAULT_ADMIN_ROLE, NilConstants.OWNER_ROLE);

        // The PROPOSER_ROLE_ADMIN are set to be managed by the DEFAULT_ADMIN_ROLE.
        // This allows the default admin to manage the committers and state updaters.
        _setRoleAdmin(NilConstants.PROPOSER_ROLE_ADMIN, DEFAULT_ADMIN_ROLE);

        // The PROPOSER_ROLE are set to be managed by their respective admin roles.
        // This allows the proposers to be managed by the roles designated for their administration.
        _setRoleAdmin(NilConstants.PROPOSER_ROLE, NilConstants.PROPOSER_ROLE_ADMIN);

        // Grant roles to defaultAdmin and owner
        // The DEFAULT_ADMIN_ROLE is granted to both the default admin and the owner to ensure that both have the
        // highest level of control.
        // The PROPOSER_ROLE_ADMIN is granted to both the default admin and the owner to allow them to manage proposers.
        // The OWNER_ROLE is granted to the owner to ensure they have the highest level of control over the contract.
        _grantRole(NilConstants.OWNER_ROLE, _owner);
        _grantRole(DEFAULT_ADMIN_ROLE, _defaultAdmin);
        _grantRole(NilConstants.PROPOSER_ROLE_ADMIN, _defaultAdmin);
        _grantRole(NilConstants.PROPOSER_ROLE_ADMIN, _owner);

        // Grant proposer to defaultAdmin and owner
        // The PROPOSER_ROLE is granted to the default admin and the owner.
        // This ensures that both the default admin and the owner have the necessary permissions to perform
        // committer and state updater functions if needed. This redundancy provides a fallback mechanism
        _grantRole(NilConstants.PROPOSER_ROLE, _owner);
        _grantRole(NilConstants.PROPOSER_ROLE, _defaultAdmin);

        // Grant PROPOSER_ROLE to proposerAddress
        if (_proposer != address(0)) {
            _grantRole(NilConstants.PROPOSER_ROLE, _proposer);
        }

        // Initialize the first batch with a dummy string and GENESIS_STATE_ROOT
        // This is necessary to set up an initial state for the rollup contract.
        // The dummy string "GENESIS_BATCH_INDEX" is used as the initial batch index, and the GENESIS_STATE_ROOT
        // is used as the initial state root. This ensures that the contract has a valid initial state
        // and can correctly handle the first batch update.
        lastFinalizedBatchIndex = GENESIS_BATCH_INDEX;

        INilRollup.PublicDataInfo memory publicDataInfo =
            INilRollup.PublicDataInfo({ l2Tol1Root: ZERO_STATE_ROOT, messageCount: 0, l1MessageHash: ZERO_STATE_ROOT });

        batchInfoRecords[lastFinalizedBatchIndex] = BatchInfo({
            batchIndex: lastFinalizedBatchIndex,
            isCommitted: true,
            isFinalized: true,
            versionedHashes: new bytes32[](0),
            oldStateRoot: ZERO_STATE_ROOT,
            newStateRoot: _genesisStateRoot,
            dataProofs: new bytes[](0),
            validityProof: "",
            publicDataInfo: publicDataInfo,
            blobCount: 0
        });

        // Initialize the stateRootIndex mapping for the _genesisStateRoot to GENESIS_BATCH_INDEX
        stateRootIndex[_genesisStateRoot] = GENESIS_BATCH_INDEX;

        nilVerifierAddress = _nilVerifierAddress;
    }

    /*//////////////////////////////////////////////////////////////////////////
                           USER-FACING CONSTANT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /// @inheritdoc INilRollup
    function getLastFinalizedBatchIndex() external view override returns (string memory) {
        return lastFinalizedBatchIndex;
    }

    /// @inheritdoc INilRollup
    function getBlobVersionedHashes(string memory batchIndex) public view override returns (bytes32[] memory) {
        return batchInfoRecords[batchIndex].versionedHashes;
    }

    /// @inheritdoc INilRollup
    function finalizedStateRoots(string memory batchIndex) external view override returns (bytes32) {
        return batchInfoRecords[batchIndex].newStateRoot;
    }

    /// @inheritdoc INilRollup
    function isBatchFinalized(string memory batchIndex) external view override returns (bool) {
        return batchInfoRecords[batchIndex].isFinalized;
    }

    /// @inheritdoc INilRollup
    function isRootFinalized(bytes32 stateRoot) external view override returns (bool) {
        return bytes(stateRootIndex[stateRoot]).length != 0;
    }

    /// @inheritdoc INilRollup
    function batchIndexOfRoot(bytes32 stateRoot) external view override returns (string memory) {
        return stateRootIndex[stateRoot];
    }

    /// @inheritdoc INilRollup
    function previousStateRoot(
        bytes32 stateRoot
    ) external view override returns (bytes32) {
        return batchInfoRecords[stateRootIndex[stateRoot]].oldStateRoot;
    }

    /// @inheritdoc INilRollup
    function isBatchCommitted(
        string memory batchIndex
    ) external view override returns (bool) {
        return batchInfoRecords[batchIndex].isCommitted;
    }

    /// @inheritdoc INilRollup
    function getCurrentStateRoot() external view override returns (bytes32) {
        return batchInfoRecords[lastFinalizedBatchIndex].newStateRoot;
    }

    /// @inheritdoc INilRollup
    function getCurrentL2ToL1Root() external view override returns (bytes32) {
        return batchInfoRecords[lastFinalizedBatchIndex].publicDataInfo.l2Tol1Root;
    }

    /*//////////////////////////////////////////////////////////////////////////
                         USER-FACING NON-CONSTANT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /// @inheritdoc INilRollup
    function commitBatch(string memory batchIndex, uint256 blobCount) external override whenNotPaused onlyProposer {
        // check if the batch is not committed and finalized yet

        if (bytes(batchIndex).length == 0) {
            revert ErrorInvalidBatchIndex();
        }

        if (batchInfoRecords[batchIndex].isFinalized) {
            revert ErrorBatchAlreadyFinalized(batchIndex);
        }

        if (batchInfoRecords[batchIndex].isCommitted) {
            revert ErrorBatchAlreadyCommitted(batchIndex);
        }

        // get the versionedHashes using the opcode blobhash for each blob index
        bytes32[] memory versionedHashes = new bytes32[](blobCount);
        for (uint256 i = 0; i < blobCount; ++i) {
            bytes32 versionedHash = getBlobHash(i);

            if (versionedHash == bytes32(0)) {
                revert ErrorInvalidVersionedHash(batchIndex, i);
            }

            versionedHashes[i] = versionedHash;
        }

        // mark the batch as committed
        batchInfoRecords[batchIndex].isCommitted = true;
        batchInfoRecords[batchIndex].versionedHashes = versionedHashes;
        batchInfoRecords[batchIndex].blobCount = blobCount;

        // emit an event for the committed batch
        emit BatchCommitted(batchIndex);
    }

    function getBlobHash(uint256 index) public view virtual returns (bytes32) {
        bytes32 versionedHash;
        assembly {
            versionedHash := blobhash(index)
        }
        return versionedHash;
    }

    /// @inheritdoc INilRollup
    function updateState(
        string memory batchIndex,
        bytes32 oldStateRoot,
        bytes32 newStateRoot,
        bytes[] calldata dataProofs,
        bytes calldata validityProof,
        PublicDataInfo calldata publicDataInfo
    )
        external
        override
        whenNotPaused
        onlyProposer
    {
        if (bytes(batchIndex).length == 0) {
            revert ErrorInvalidBatchIndex();
        }

        // Check if oldStateRoot and newStateRoot are valid values
        if (oldStateRoot == bytes32(0)) {
            revert ErrorInvalidOldStateRoot();
        }

        if (newStateRoot == bytes32(0)) {
            revert ErrorInvalidNewStateRoot();
        }

        if (validityProof.length == 0) {
            revert ErrorInvalidValidityProof();
        }

        // Check if dataProofs and validityProof are not zero values
        if (dataProofs.length == 0) {
            revert ErrorEmptyDataProofs();
        }

        // Check if batchIndex has storage values of isCommitted true and isFinalized false
        if (!batchInfoRecords[batchIndex].isCommitted) {
            revert ErrorBatchNotCommitted(batchIndex);
        }

        if (batchInfoRecords[batchIndex].isFinalized) {
            revert ErrorBatchAlreadyFinalized(batchIndex);
        }

        // Check if dataProofs length matches the blobCount
        if (dataProofs.length != batchInfoRecords[batchIndex].blobCount) {
            revert ErrorDataProofsAndBlobCountMismatch(dataProofs.length, batchInfoRecords[batchIndex].blobCount);
        }

        // check if newStateRoot is not finalized
        if (bytes(stateRootIndex[newStateRoot]).length != 0) {
            revert ErrorNewStateRootAlreadyFinalized(batchIndex, newStateRoot);
        }

        // Check if the oldStateRoot matches the last finalized batch's newStateRoot
        if (batchInfoRecords[lastFinalizedBatchIndex].newStateRoot != oldStateRoot) {
            revert ErrorOldStateRootMismatch();
        }

        bytes32[] memory blobVersionedHashes = getBlobVersionedHashes(batchIndex);

        if (blobVersionedHashes.length != dataProofs.length) {
            revert ErrorIncorrectDataProofSize();
        }

        // get the messageCount from the publicInput
        uint256 depositMessageCount = publicDataInfo.messageCount;

        if (
            (
                depositMessageCount == 0
                    && (
                        publicDataInfo.l2Tol1Root != batchInfoRecords[lastFinalizedBatchIndex].publicDataInfo.l2Tol1Root
                            || publicDataInfo.l1MessageHash != ZERO_STATE_ROOT
                    )
            )
                || (
                    (
                        (
                            depositMessageCount > 0
                                && (
                                    (
                                        (
                                            publicDataInfo.l2Tol1Root
                                                == batchInfoRecords[lastFinalizedBatchIndex].publicDataInfo.l2Tol1Root
                                        )
                                    ) || publicDataInfo.l1MessageHash == ZERO_STATE_ROOT
                                )
                        )
                    )
                )
        ) {
            revert ErrorInvalidPublicDataInfo();
        }

        if (depositMessageCount > 0) {
            // pull first n messages from the messageQueue via l1BridgeMessenger
            bytes32[] memory depositMessageHashes = l1BridgeMessenger.popMessages(depositMessageCount);

            if (l1MessageHash == bytes32(0)) {
                if (depositMessageCount == 1) {
                    l1MessageHash = depositMessageHashes[0];
                } else {
                    bytes32 l1MessageHashLocal = depositMessageHashes[0];
                    for (uint256 messageHashIndex = 1; messageHashIndex < depositMessageCount; messageHashIndex++) {
                        l1MessageHashLocal =
                            keccak256(abi.encode(l1MessageHashLocal, depositMessageHashes[messageHashIndex]));
                    }
                    l1MessageHash = l1MessageHashLocal;
                }
            } else {
                bytes32 l1MessageHashLocal = l1MessageHash;
                for (uint256 messageHashIndex = 0; messageHashIndex < depositMessageCount; messageHashIndex++) {
                    l1MessageHashLocal =
                        keccak256(abi.encode(depositMessageHashes[messageHashIndex], l1MessageHashLocal));
                }
                l1MessageHash = l1MessageHashLocal;
            }

            // Check if the l1MessageHash in publicDataInput is the same as the l1MessageHash computed above
            if (l1MessageHash != publicDataInfo.l1MessageHash) {
                revert ErrorL1MessageHashMismatch(l1MessageHash, publicDataInfo.l1MessageHash);
            }
        }

        for (uint256 i = 0; i < blobVersionedHashes.length; i++) {
            if (dataProofs[i].length == 0) {
                revert ErrorInvalidDataProofItem(i);
            }

            verifyDataProof(blobVersionedHashes[i], dataProofs[i]);
        }

        // generate publicInput for validityProof Verification
        bytes memory publicInput = generatePublicInputForValidityProofVerification(batchIndex, publicDataInfo);

        if (publicInput.length == 0) {
            revert ErrorInvalidPublicInputForProof();
        }

        // verify the validityProof generated by circuit
        INilVerifier(nilVerifierAddress).verify(validityProof, publicInput);

        // update state root
        batchInfoRecords[batchIndex].oldStateRoot = oldStateRoot;
        batchInfoRecords[batchIndex].newStateRoot = newStateRoot;

        for (uint256 i = 0; i < dataProofs.length; i++) {
            batchInfoRecords[batchIndex].dataProofs.push(dataProofs[i]);
        }

        batchInfoRecords[batchIndex].validityProof = validityProof;
        batchInfoRecords[batchIndex].publicDataInfo = publicDataInfo;
        batchInfoRecords[batchIndex].isFinalized = true;

        // update state root index
        stateRootIndex[newStateRoot] = batchIndex;

        lastFinalizedBatchIndex = batchIndex;

        // emit an event for the updated state root
        emit StateRootUpdated(batchIndex, oldStateRoot, newStateRoot);
    }

    /// @inheritdoc INilRollup
    function verifyDataProof(bytes32 blobVersionedHash, bytes calldata dataProof) public view {
        // Calls the point evaluation precompile and verifies the output
        (bool success, bytes memory data) =
            POINT_EVALUATION_PRECOMPILE_ADDR.staticcall(abi.encodePacked(blobVersionedHash, dataProof));
        // verify that the point evaluation precompile call was successful by testing the latter 32 bytes of the
        // response is equal to BLS_MODULUS as defined in
        // https://eips.ethereum.org/EIPS/eip-4844#point-evaluation-precompile
        if (!success) revert ErrorCallPointEvaluationPrecompileFailed();
        (, uint256 result) = abi.decode(data, (uint256, uint256));
        if (result != BLS_MODULUS) {
            revert ErrorUnexpectedPointEvaluationPrecompileOutput();
        }
    }

    /// @inheritdoc INilRollup
    function generatePublicInputForValidityProofVerification(
        string memory batchIndex,
        PublicDataInfo calldata publicDataInfo
    )
        public
        view
        virtual
        returns (bytes memory)
    {
        return hex"0000dead";
    }

    /*//////////////////////////////////////////////////////////////////////////
                         RESTRICTED FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

    /// @inheritdoc INilRollup
    function setPause(bool _status) external onlyOwner {
        if (_status) {
            _pause();
        } else {
            _unpause();
        }
    }

    /// @inheritdoc INilRollup
    function transferOwnershipRole(address newOwner) external override onlyOwner {
        _revokeRole(NilConstants.OWNER_ROLE, owner());
        super.transferOwnership(newOwner);
        _grantRole(NilConstants.OWNER_ROLE, newOwner);
    }

    /**
     * @dev Sets the address of the NilVerifier contract.
     * @param nilVerifier The new address of the NilVerifier contract.
     */
    function setVerifierAddress(address nilVerifier) external onlyAdmin {
        if (nilVerifier == address(0)) {
            revert ErrorInvalidNilVerifier();
        }

        if (nilVerifier == nilVerifierAddress) {
            revert ErrorNilVerifierAddressNotChanged();
        }

        nilVerifierAddress = nilVerifier;
    }

    /**
     * @notice Resets the contract state to the specified state root and removes associated committed batches.
     * @dev Removes all state roots and their corresponding committed batches from the mappings and history,
     *      starting from the given `targetStateRoot`. This function handles only the batches that are directly
     *      linked to valid state roots, while ignoring any unrelated or dangling batch entries.
     * @param targetStateRoot The state root to which the state will be reset. If this state root does not exist
     *        in the stateRootIndex, an error is thrown.
     */
    function resetState(bytes32 targetStateRoot) external onlyAdmin {
        // Check if targetStateRoot is a valid value
        if (targetStateRoot == bytes32(0)) {
            revert ErrorInvalidResetStateRoot();
        }

        // Check if the targetStateRoot exists in the mapping
        if (bytes(stateRootIndex[targetStateRoot]).length == 0) {
            revert ErrorResetStateRootNotFound();
        }

        // Start from the last finalized batch and move backwards
        bytes32 currentStateRoot = batchInfoRecords[lastFinalizedBatchIndex].newStateRoot;
        while (currentStateRoot != targetStateRoot) {
            bytes32 oldStateRoot = this.previousStateRoot(currentStateRoot);

            // Delete the current state root record and associated batch
            delete batchInfoRecords[stateRootIndex[currentStateRoot]];
            delete stateRootIndex[currentStateRoot];

            // Move to the previous state root in the chain
            currentStateRoot = oldStateRoot;
        }

        // Update the last finalized batch index to the reset point
        lastFinalizedBatchIndex = stateRootIndex[currentStateRoot];

        emit StateReset(targetStateRoot);
    }

    function setL1BridgeMessenger(address l1BridgeMessengerAddress) external onlyAdmin {
        if (l1BridgeMessengerAddress == address(0)) {
            revert ErrorInvalidAddress();
        }

        l1BridgeMessenger = IL1BridgeMessenger(l1BridgeMessengerAddress);
    }
}
