// SPDX-License-Identifier: MIT
pragma solidity 0.8.27;

import { stdJson } from "forge-std/Test.sol";
import { PausableUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import { BaseTest } from "./BaseTest.sol";
import { NilRollup } from "../contracts/NilRollup.sol";
import { NilAccessControl } from "../contracts/NilAccessControl.sol";
import { NilRollupMockBlob } from "./mocks/NilRollupMockBlob.sol";
import { NilRollupMockBlobInvalidScenario } from "./mocks/NilRollupMockBlobInvalidScenario.sol";
import { ITransparentUpgradeableProxy } from "@openzeppelin/contracts/proxy/transparent/TransparentUpgradeableProxy.sol";

contract NilRollupTest is BaseTest {
    using stdJson for string;

    function setUp() public override {
        super.setUp();
    }

    /**
     * @notice Tests the `initialize` function to ensure it reverts when called after the contract has already been
     * initialized.
     *
     * @dev This test follows these steps:
     * 1. Asserts that the owner of the rollup contract is correctly set.
     * 2. Asserts that the L2 chain ID is correctly set to 0.
     * 3. Attempts to initialize the rollup contract again, expecting a revert due to invalid initialization.
     *
     * The test ensures that the `initialize` function correctly handles the scenario where the contract has already
     * been initialized, preventing reinitialization.
     */
    function test_initialized() external {
        assertEq(_owner, rollup.owner());
        assertEq(rollup.l2ChainId(), 0);

        bytes32 blobHash = NilRollupMockBlob(address(rollup)).getBlobHash(0);

        vm.expectRevert(abi.encodeWithSelector(InvalidInitialization.selector));
        rollup.initialize(_l2ChainId, _owner, _defaultAdmin, address(nilVerifier), _proposer, _genesisStateRoot);
    }

    /**
     * @notice Tests the `getBlobHash` function to ensure it returns the correct blob hashes for given indices.
     *
     * @dev This test follows these steps:
     * 1. Prepares dummy blob-hash data for 3 blobs.
     * 2. Sets the dummy blob hashes in the mock data for corresponding blob indices.
     * 3. Asserts the blob hashes by querying using the `getBlobHash` function at specific indices.
     *
     * The test ensures that the `getBlobHash` function correctly returns the blob hashes for given indices.
     */
    function test_getBlobHash() external {
        // Prepare dummy blob-hash data for 3 blobs
        bytes32[] memory dummyBlobHashes = new bytes32[](3);
        dummyBlobHashes[0] = keccak256(abi.encodePacked("dummyBlob1"));
        dummyBlobHashes[1] = keccak256(abi.encodePacked("dummyBlob2"));
        dummyBlobHashes[2] = keccak256(abi.encodePacked("dummyBlob3"));

        // Set them in the mock data for corresponding blob indices
        for (uint256 i = 0; i < dummyBlobHashes.length; i++) {
            NilRollupMockBlob(address(rollup)).setBlobVersionedHash(i, dummyBlobHashes[i]);
        }

        // Assert them by querying using getBlobHash function at specific indices
        for (uint256 i = 0; i < dummyBlobHashes.length; i++) {
            bytes32 blobHash = NilRollupMockBlob(address(rollup)).getBlobHash(i);
            assertEq(blobHash, dummyBlobHashes[i]);
        }
    }

    /**
     * @notice Tests the `verifyDataProof` function to ensure it correctly verifies the data proof for a given blob
     * versioned hash.
     *
     * @dev This test follows these steps:
     * 1. Sets a blob versioned hash and corresponding data proof.
     * 2. Calls the `verifyDataProof` function with the blob versioned hash and data proof.
     *
     * The test ensures that the `verifyDataProof` function correctly verifies the data proof for the given blob
     * versioned hash.
     */
    function test_verifyDataProof() external {
        bytes32 blobVersionedHash = 0x0177796aa994d21fd2c64b554ea78bb7079c3adb026ef79bbadee107d87ae1a4;
        bytes memory blobDataProof =
            hex"3d8f2613194608a6f844c82489287f591b3b270147c4e18fde1cc8fcd093e869557571e10918383bc34de6487e41c65c99a44d672065d1dc02e322923584da23b48ec3bfaec9dfa9a2e7cf5fb31a5bd5a8daa222b64d712d410880f7a62cd343fb03ca93da88a5939a594a57a80b36cb96a2acb932cc66b5951ccf8289ca57e8d977dce8dcd900f63f15cdb6bf0233ccc1fb920f187b43c5db964a50b6c232fa";
        NilRollupMockBlob(address(rollup)).verifyDataProof(blobVersionedHash, blobDataProof);
    }

    /**
     * @notice Tests the `commitBatch` function to ensure it correctly commits a batch with test data.
     *
     * @dev This test follows these steps:
     * 1. Calls the `commitBatchWithTestData` function to commit a batch with predefined test data.
     *
     * The test ensures that the `commitBatch` function correctly handles the scenario of committing a batch with test
     * data.
     */
    function test_commitBatch() public {
        commitBatchWithTestData(_proposer);
    }

    /**
     * @notice Tests the `updateState` function to ensure it correctly updates the state with test data.
     *
     * @dev This test follows these steps:
     * 1. Calls the `commitBatchWithTestData` function to commit a batch with predefined test data.
     * 2. Calls the `updateStateWithTestData` function to update the state with the committed batch data.
     *
     * The test ensures that the `updateState` function correctly handles the scenario of updating the state with test
     * data.
     */
    function test_updateState() external {
        BatchData memory batchData = commitBatchWithTestData(_proposer);
        updateStateWithTestData(batchData);
    }

    /**
     * @notice Tests the `commitBatch` function to ensure it reverts when called by a non-proposer.
     *
     * @dev This test follows these steps:
     * 1. Sets a valid versioned hash for the first batch.
     * 2. Expects a revert due to the caller not being the proposer.
     * 3. Attempts to commit the batch, expecting a revert due to the caller not being the proposer.
     *
     * The test ensures that the `commitBatch` function correctly handles the scenario where the caller is not the
     * proposer, preventing unauthorized commits.
     */
    function test_commitBatch_toRevert_by_nonProposer() public {
        // Set a valid versioned hash for the first batch
        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);

        // Expect a revert due to the caller not being the proposer
        vm.expectRevert(NilAccessControl.ErrorCallerIsNotProposer.selector);

        // Attempt to commit the batch, expecting a revert due to the caller not being the proposer
        rollup.commitBatch("BATCH_1", 1);
    }

    /**
     * @notice Tests the `commitBatch` function to ensure it reverts when the batch index is invalid (empty).
     *
     * @dev This test follows these steps:
     * 1. Sets a valid versioned hash for the first batch.
     * 2. Starts a prank as the proposer.
     * 3. Expects a revert due to the invalid (empty) batch index.
     * 4. Attempts to commit the batch with the invalid batch index, expecting a revert due to the invalid batch index.
     *
     * The test ensures that the `commitBatch` function correctly handles the scenario where the batch index is invalid,
     * preventing commits with incorrect batch indices.
     */
    function test_commitBatch_toRevert_with_invalid_batchIndex() public {
        // Set a valid versioned hash for the first batch
        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);

        // Start a prank as the proposer
        vm.startPrank(_proposer);

        // Expect a revert due to the invalid (empty) batch index
        vm.expectRevert(NilRollup.ErrorInvalidBatchIndex.selector);

        // Attempt to commit the batch with the invalid batch index
        rollup.commitBatch("", 1);

        // Stop the prank
        vm.stopPrank();
    }

    /**
     * @notice Tests the `commitBatch` function to ensure it reverts when the versioned hash is invalid (empty).
     *
     * @dev This test follows these steps:
     * 1. Sets an invalid (empty) versioned hash for the first batch.
     * 2. Starts a prank as the proposer.
     * 3. Attempts to commit the batch with the invalid versioned hash, expecting a revert due to the invalid versioned
     * hash.
     *
     * The test ensures that the `commitBatch` function correctly handles the scenario where the versioned hash is
     * invalid, preventing commits with incorrect versioned hashes.
     */
    function test_commitBatch_toRevert_with_invalid_VersionedHash() public {
        // Set an invalid (empty) versioned hash for the first batch
        bytes32 versionedHash = hex"";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);

        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 1;

        // Start a prank as the proposer
        vm.startPrank(_proposer);

        // Expect a revert due to the invalid versioned hash
        vm.expectRevert(abi.encodeWithSelector(NilRollup.ErrorInvalidVersionedHash.selector, batchIndex, 0));

        // Attempt to commit the batch with the invalid versioned hash
        rollup.commitBatch(batchIndex, blobCount);

        // Stop the prank
        vm.stopPrank();
    }

    /**
     * @notice Tests the `commitBatch` function to ensure it reverts when attempting to commit a batch that has already
     * been committed.
     *
     * @dev This test follows these steps:
     * 1. Starts a prank as the proposer.
     * 2. Sets a valid versioned hash for the first batch.
     * 3. Commits the first batch and verifies its committed state.
     * 4. Attempts to commit the same batch again, expecting a revert due to the batch already being committed.
     *
     * The test ensures that the `commitBatch` function correctly handles the scenario where a batch has already been
     * committed, preventing duplicate commits.
     */
    function test_commitBatch_toRevert_duplicateCommit() public {
        // Start a prank as the proposer
        vm.startPrank(_proposer);

        // Set a valid versioned hash for the first batch
        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);
        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 1;

        // Commit the first batch
        rollup.commitBatch(batchIndex, blobCount);

        // Verify the committed state of the first batch
        assertTrue(rollup.isBatchCommitted(batchIndex));
        assertFalse(rollup.isBatchFinalized(batchIndex));

        // Verify the last committed batch index
        string memory lastCommittedBatchIndex = rollup.getLastCommittedBatchIndex();
        assertEq(lastCommittedBatchIndex, batchIndex);

        // Expect a revert due to the batch already being committed
        vm.expectRevert(abi.encodeWithSelector(NilRollup.ErrorBatchAlreadyCommitted.selector, "BATCH_1"));

        // Attempt to commit the same batch again
        rollup.commitBatch(batchIndex, blobCount);

        // Stop the prank
        vm.stopPrank();
    }

    /**
     * @notice Tests the `commitBatch` function to ensure it reverts when attempting to commit a batch that has already
     * been finalized.
     *
     * @dev This test follows these steps:
     * 1. Starts a prank as the proposer.
     * 2. Sets a blob versioned hash for the first batch.
     * 3. Commits the first batch and verifies its committed state.
     * 4. Prepares state update details for the first batch.
     * 5. Updates the state with the first batch's details, finalizing the batch.
     * 6. Attempts to commit the same batch again, expecting a revert due to the batch already being finalized.
     *
     * The test ensures that the `commitBatch` function correctly handles the scenario where a batch has already been
     * finalized, preventing duplicate commits.
     */
    function test_commitBatch_toRevert_when_commit_a_finalizedBatch() public {
        // Start a prank as the proposer
        vm.startPrank(_proposer);

        // Set a blob versioned hash for the first batch
        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);
        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 1;

        // Commit the first batch
        rollup.commitBatch(batchIndex, blobCount);

        // Verify the committed state of the first batch
        assertTrue(rollup.isBatchCommitted(batchIndex));
        assertFalse(rollup.isBatchFinalized(batchIndex));

        // Prepare state update details for the first batch
        bytes32 oldStateRoot = hex"8de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes32 newStateRoot = hex"9de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes[] memory dataProofs = new bytes[](1);
        dataProofs[0] =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";
        bytes memory validityProof =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";

        // Update the state with the first batch's details
        rollup.updateState(batchIndex, oldStateRoot, newStateRoot, dataProofs, validityProof, publicDataInfoMock);

        // Expect a revert due to the batch already being finalized
        vm.expectRevert(abi.encodeWithSelector(NilRollup.ErrorBatchAlreadyFinalized.selector, "BATCH_1"));
        rollup.commitBatch(batchIndex, blobCount);

        // Stop the prank
        vm.stopPrank();
    }

    /**
     * @notice Tests the `updateState` function to ensure it reverts when the old state root is invalid (empty).
     *
     * @dev This test follows these steps:
     * 1. Starts a prank as the proposer.
     * 2. Sets a blob versioned hash for the first batch.
     * 3. Commits the first batch and verifies its committed state.
     * 4. Prepares state update details for the first batch with an invalid (empty) old state root.
     * 5. Attempts to update the state with the first batch's details, expecting a revert due to the invalid old state
     * root.
     *
     * The test ensures that the `updateState` function correctly handles the scenario where the old state root is
     * invalid, preventing state updates with incorrect state roots.
     */
    function test_updateState_toRevert_with_invalid_oldStateRoot() external {
        // Start a prank as the proposer
        vm.startPrank(_proposer);

        // Set a blob versioned hash for the first batch
        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);
        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 1;

        // Commit the first batch
        rollup.commitBatch(batchIndex, blobCount);

        // Verify the committed state of the first batch
        assertTrue(rollup.isBatchCommitted(batchIndex));
        assertFalse(rollup.isBatchFinalized(batchIndex));

        // Prepare state update details for the first batch with an invalid (empty) old state root
        bytes32 oldStateRoot = hex"";
        bytes32 newStateRoot = hex"9de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes[] memory dataProofs = new bytes[](1);
        dataProofs[0] =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";
        bytes memory validityProof =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";

        // Expect a revert due to the invalid old state root
        vm.expectRevert(abi.encodeWithSelector(NilRollup.ErrorInvalidOldStateRoot.selector));

        // Attempt to update the state with the first batch's details
        rollup.updateState(batchIndex, oldStateRoot, newStateRoot, dataProofs, validityProof, publicDataInfoMock);

        // Stop the prank
        vm.stopPrank();
    }

    /**
     * @notice Tests the `updateState` function to ensure it reverts when the old state root does not match the expected
     * state root.
     *
     * @dev This test follows these steps:
     * 1. Starts a prank as the proposer.
     * 2. Sets a blob versioned hash for the first batch.
     * 3. Commits the first batch and verifies its committed state.
     * 4. Prepares state update details for the first batch with an old state root that does not match the expected
     * state root.
     * 5. Attempts to update the state with the first batch's details, expecting a revert due to the old state root
     * mismatch.
     *
     * The test ensures that the `updateState` function correctly handles the scenario where the old state root does not
     * match the expected state root, preventing state updates with incorrect state roots.
     */
    function test_updateState_toRevert_with_oldStateRoot_mismatch() external {
        // Start a prank as the proposer
        vm.startPrank(_proposer);

        // Set a blob versioned hash for the first batch
        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);
        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 1;

        // Commit the first batch
        rollup.commitBatch(batchIndex, blobCount);

        // Verify the committed state of the first batch
        assertTrue(rollup.isBatchCommitted(batchIndex));
        assertFalse(rollup.isBatchFinalized(batchIndex));

        // Prepare state update details for the first batch with an old state root that does not match the expected
        // state root
        bytes32 oldStateRoot = hex"9de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes32 newStateRoot = hex"9de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes[] memory dataProofs = new bytes[](1);
        dataProofs[0] =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";
        bytes memory validityProof =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";

        // Expect a revert due to the old state root mismatch
        vm.expectRevert(abi.encodeWithSelector(NilRollup.ErrorOldStateRootMismatch.selector));

        // Attempt to update the state with the first batch's details
        rollup.updateState(batchIndex, oldStateRoot, newStateRoot, dataProofs, validityProof, publicDataInfoMock);

        // Stop the prank
        vm.stopPrank();
    }

    /**
     * @notice Tests the `updateState` function to ensure it reverts when the new state root is invalid (empty).
     *
     * @dev This test follows these steps:
     * 1. Starts a prank as the proposer.
     * 2. Sets a blob versioned hash for the first batch.
     * 3. Commits the first batch and verifies its committed state.
     * 4. Prepares state update details for the first batch with an invalid (empty) new state root.
     * 5. Attempts to update the state with the first batch's details, expecting a revert due to the invalid new state
     * root.
     *
     * The test ensures that the `updateState` function correctly handles the scenario where the new state root is
     * invalid, preventing state updates with incorrect state roots.
     */
    function test_updateState_toRevert_with_invalid_newStateRoot() external {
        // Start a prank as the proposer
        vm.startPrank(_proposer);

        // Set a blob versioned hash for the first batch
        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);
        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 1;

        // Commit the first batch
        rollup.commitBatch(batchIndex, blobCount);

        // Verify the committed state of the first batch
        assertTrue(rollup.isBatchCommitted(batchIndex));
        assertFalse(rollup.isBatchFinalized(batchIndex));

        // Prepare state update details for the first batch with an invalid (empty) new state root
        bytes32 oldStateRoot = hex"8de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes32 newStateRoot = hex"";
        bytes[] memory dataProofs = new bytes[](1);
        dataProofs[0] =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";
        bytes memory validityProof =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";

        // Expect a revert due to the invalid new state root
        vm.expectRevert(abi.encodeWithSelector(NilRollup.ErrorInvalidNewStateRoot.selector));

        // Attempt to update the state with the first batch's details
        rollup.updateState(batchIndex, oldStateRoot, newStateRoot, dataProofs, validityProof, publicDataInfoMock);

        // Stop the prank
        vm.stopPrank();
    }

    /**
     * @notice Tests the `updateState` function to ensure it reverts when the data proofs array is empty.
     *
     * @dev This test follows these steps:
     * 1. Starts a prank as the proposer.
     * 2. Sets a blob versioned hash for the first batch.
     * 3. Commits the first batch and verifies its committed state.
     * 4. Prepares state update details for the first batch with an invalid (empty) data proof.
     * 5. Attempts to update the state with the first batch's details, expecting a revert due to the invalid data proof.
     *
     * The test ensures that the `updateState` function correctly handles the scenario where the data proofs array is
     * empty, preventing state updates with incorrect data proofs.
     */
    function test_updateState_toRevert_With_EmptyDataProofs() external {
        // Start a prank as the proposer
        vm.startPrank(_proposer);

        // Set a blob versioned hash for the first batch
        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);
        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 1;

        // Commit the first batch
        rollup.commitBatch(batchIndex, blobCount);

        // Verify the committed state of the first batch
        assertTrue(rollup.isBatchCommitted(batchIndex));
        assertFalse(rollup.isBatchFinalized(batchIndex));

        // Prepare state update details for the first batch with an invalid (empty) data proof
        bytes32 oldStateRoot = hex"8de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes32 newStateRoot = hex"9de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes[] memory dataProofs = new bytes[](0);
        bytes memory validityProof =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";

        // Expect a revert due to the invalid data proof
        vm.expectRevert(abi.encodeWithSelector(NilRollup.ErrorEmptyDataProofs.selector));

        // Attempt to update the state with the first batch's details
        rollup.updateState(batchIndex, oldStateRoot, newStateRoot, dataProofs, validityProof, publicDataInfoMock);

        // Stop the prank
        vm.stopPrank();
    }

    /**
     * @notice Tests the `updateState` function to ensure it reverts when the data proofs array length does not match
     * the blob count.
     *
     * @dev This test follows these steps:
     * 1. Starts a prank as the proposer.
     * 2. Sets blob versioned hashes for the first batch.
     * 3. Commits the first batch and verifies its committed state.
     * 4. Prepares state update details for the first batch with mismatched data proofs array length and blob count.
     * 5. Attempts to update the state with the first batch's details, expecting a revert due to the mismatch.
     *
     * The test ensures that the `updateState` function correctly handles the scenario where the data proofs array
     * length does not match the blob count, preventing state updates with incorrect data proofs.
     */
    function test_revert_when_updateState_with_dataProofsAndBlobCountMismatch() external {
        // Start a prank as the proposer
        vm.startPrank(_proposer);

        // Set blob versioned hashes for the first batch
        bytes32 versionedHash_0 = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash_0);

        bytes32 versionedHash_1 = hex"01a1cf2318c1a60915f77b2b004241dfcddaf7a98971c6b087c93b04a3b4e638";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(1, versionedHash_1);

        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 2;

        // Commit the first batch
        rollup.commitBatch(batchIndex, blobCount);

        // Verify the committed state of the first batch
        assertTrue(rollup.isBatchCommitted(batchIndex));
        assertFalse(rollup.isBatchFinalized(batchIndex));

        // Prepare state update details for the first batch with mismatched data proofs array length and blob count
        bytes32 oldStateRoot = hex"8de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes32 newStateRoot = hex"9de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes[] memory dataProofs = new bytes[](1);
        dataProofs[0] =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";
        bytes memory validityProof =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";

        // Expect a revert due to the mismatch
        vm.expectRevert(
            abi.encodeWithSelector(NilRollup.ErrorDataProofsAndBlobCountMismatch.selector, dataProofs.length, blobCount)
        );

        // Attempt to update the state with the first batch's details
        rollup.updateState(batchIndex, oldStateRoot, newStateRoot, dataProofs, validityProof, publicDataInfoMock);

        // Stop the prank
        vm.stopPrank();
    }

    /**
     * @notice Tests the `updateState` function to ensure it reverts when attempting to update the state for a
     * non-committed batch.
     *
     * @dev This test follows these steps:
     * 1. Starts a prank as the proposer.
     * 2. Sets a blob versioned hash for the first batch.
     * 3. Verifies the committed state of the first batch (should be false).
     * 4. Prepares state update details for the first batch.
     * 5. Attempts to update the state with the first batch's details, expecting a revert due to the batch not being
     * committed.
     *
     * The test ensures that the `updateState` function correctly handles the scenario where the batch has not been
     * committed, preventing state updates for non-committed batches.
     */
    function test_revert_when_updateState_on_nonCommittedBatch() external {
        // Start a prank as the proposer
        vm.startPrank(_proposer);

        // Set a blob versioned hash for the first batch
        bytes32 versionedHash_0 = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash_0);

        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 1;

        // Verify the committed state of the first batch
        assertFalse(rollup.isBatchCommitted(batchIndex));
        assertFalse(rollup.isBatchFinalized(batchIndex));

        // Prepare state update details for the first batch
        bytes32 oldStateRoot = hex"8de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes32 newStateRoot = hex"9de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes[] memory dataProofs = new bytes[](1);
        dataProofs[0] =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";
        bytes memory validityProof =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";

        // Expect a revert due to the batch not being committed
        vm.expectRevert(abi.encodeWithSelector(NilRollup.ErrorBatchNotCommitted.selector, batchIndex));

        // Attempt to update the state with the first batch's details
        rollup.updateState(batchIndex, oldStateRoot, newStateRoot, dataProofs, validityProof, publicDataInfoMock);

        // Stop the prank
        vm.stopPrank();
    }

    /**
     * @notice Tests the `updateState` function to ensure it reverts when the data proof is invalid.
     *
     * @dev This test follows these steps:
     * 1. Starts a prank as the proposer.
     * 2. Sets a blob versioned hash for the first batch.
     * 3. Commits the first batch and verifies its committed state.
     * 4. Prepares state update details for the first batch with an invalid (empty) data proof.
     * 5. Attempts to update the state with the first batch's details, expecting a revert due to the invalid data proof.
     *
     * The test ensures that the `updateState` function correctly handles the scenario where the data proof is invalid,
     * preventing state updates with incorrect proofs.
     */
    function test_revert_when_updateState_with_invalid_dataProof() external {
        // Start a prank as the proposer
        vm.startPrank(_proposer);

        // Set a blob versioned hash for the first batch
        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);
        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 1;

        // Commit the first batch
        rollup.commitBatch(batchIndex, blobCount);

        // Verify the committed state of the first batch
        assertTrue(rollup.isBatchCommitted(batchIndex));
        assertFalse(rollup.isBatchFinalized(batchIndex));

        // Prepare state update details for the first batch with an invalid (empty) data proof
        bytes32 oldStateRoot = hex"8de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes32 newStateRoot = hex"9de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes[] memory dataProofs = new bytes[](1);
        dataProofs[0] = hex"";
        bytes memory validityProof =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";

        // Expect a revert due to the invalid data proof
        vm.expectRevert(abi.encodeWithSelector(NilRollup.ErrorInvalidDataProofItem.selector, 0));

        // Attempt to update the state with the first batch's details
        rollup.updateState(batchIndex, oldStateRoot, newStateRoot, dataProofs, validityProof, publicDataInfoMock);

        // Stop the prank
        vm.stopPrank();
    }

    /**
     * @notice Tests the `updateState` function to ensure it reverts when the validity proof is invalid.
     *
     * @dev This test follows these steps:
     * 1. Starts a prank as the proposer.
     * 2. Sets a blob versioned hash for the first batch.
     * 3. Commits the first batch and verifies its committed state.
     * 4. Prepares state update details for the first batch with an invalid (empty) validity proof.
     * 5. Attempts to update the state with the first batch's details, expecting a revert due to the invalid validity
     * proof.
     *
     * The test ensures that the `updateState` function correctly handles the scenario where the validity proof is
     * invalid, preventing state updates with incorrect proofs.
     */
    function test_revert_when_updateState_with_invalid_validityProof() external {
        // Start a prank as the proposer
        vm.startPrank(_proposer);

        // Set a blob versioned hash for the first batch
        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);
        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 1;

        // Commit the first batch
        rollup.commitBatch(batchIndex, blobCount);

        // Verify the committed state of the first batch
        assertTrue(rollup.isBatchCommitted(batchIndex));
        assertFalse(rollup.isBatchFinalized(batchIndex));

        // Prepare state update details for the first batch with an invalid (empty) validity proof
        bytes32 oldStateRoot = hex"8de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes32 newStateRoot = hex"9de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes[] memory dataProofs = new bytes[](1);
        dataProofs[0] =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";
        bytes memory validityProof = hex"";

        // Expect a revert due to the invalid validity proof
        vm.expectRevert(abi.encodeWithSelector(NilRollup.ErrorInvalidValidityProof.selector));

        // Attempt to update the state with the first batch's details
        rollup.updateState(batchIndex, oldStateRoot, newStateRoot, dataProofs, validityProof, publicDataInfoMock);

        // Stop the prank
        vm.stopPrank();
    }

    /**
     * @notice Tests the `updateState` function to ensure it reverts when attempting to update the state with a batch
     * that has already been finalized.
     *
     * @dev This test follows these steps:
     * 1. Starts a prank as the proposer.
     * 2. Sets a blob versioned hash for the first batch.
     * 3. Commits the first batch and verifies its committed state.
     * 4. Prepares state update details for the first batch.
     * 5. Updates the state with the first batch's details.
     * 6. Attempts to update the state again with the same batch's details, expecting a revert due to the batch already
     * being finalized.
     *
     * The test ensures that the `updateState` function correctly handles the scenario where the batch has already been
     * finalized, preventing duplicate state updates.
     */
    function test_updateState_toRevert_with_batch_already_finalized() external {
        // Start a prank as the proposer
        vm.startPrank(_proposer);

        // Set a blob versioned hash for the first batch
        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);
        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 1;

        // Commit the first batch
        rollup.commitBatch(batchIndex, blobCount);

        // Verify the committed state of the first batch
        assertTrue(rollup.isBatchCommitted(batchIndex));
        assertFalse(rollup.isBatchFinalized(batchIndex));

        // Prepare state update details for the first batch
        bytes32 oldStateRoot = hex"8de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes32 newStateRoot = hex"9de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes[] memory dataProofs = new bytes[](1);
        dataProofs[0] =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";
        bytes memory validityProof =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";

        // Update the state with the first batch's details
        rollup.updateState(batchIndex, oldStateRoot, newStateRoot, dataProofs, validityProof, publicDataInfoMock);

        // Expect a revert due to the batch already being finalized
        vm.expectRevert(abi.encodeWithSelector(NilRollup.ErrorBatchAlreadyFinalized.selector, batchIndex));
        rollup.updateState(batchIndex, oldStateRoot, newStateRoot, dataProofs, validityProof, publicDataInfoMock);

        // Stop the prank
        vm.stopPrank();
    }

    /**
     * @notice Tests the `updateState` function to ensure it reverts when the public input proof is invalid.
     *
     * @dev This test follows these steps:
     * 1. Starts a prank as the owner to upgrade the rollup contract to an invalid scenario mock.
     * 2. Starts a prank as the proposer.
     * 3. Sets a blob versioned hash for the first batch.
     * 4. Commits the first batch and verifies its committed state.
     * 5. Prepares state update details for the first batch.
     * 6. Attempts to update the state with the first batch's details, expecting a revert due to invalid public input
     * proof.
     *
     * The test ensures that the `updateState` function correctly handles the scenario where the public input proof is
     * invalid, preventing state updates with incorrect proofs.
     */
    function test_updateState_toRevert_invalid_publicInputProof() external {
        // Start a prank as the owner to upgrade the rollup contract to an invalid scenario mock
        vm.startPrank(_owner);
        proxyAdmin.upgradeAndCall(
            ITransparentUpgradeableProxy(address(rollup)), address(nilRollupMockBlobInvalidScenario), new bytes(0)
        );
        vm.stopPrank();

        // Start a prank as the proposer
        vm.startPrank(_proposer);

        // Set a blob versioned hash for the first batch
        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);
        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 1;

        // Commit the first batch
        rollup.commitBatch(batchIndex, blobCount);

        // Verify the committed state of the first batch
        assertTrue(rollup.isBatchCommitted(batchIndex));
        assertFalse(rollup.isBatchFinalized(batchIndex));

        // Prepare state update details for the first batch
        bytes32 oldStateRoot = hex"8de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes32 newStateRoot = hex"9de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes[] memory dataProofs = new bytes[](1);
        dataProofs[0] =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";
        bytes memory validityProof =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";

        // Expect a revert due to invalid public input proof
        vm.expectRevert(abi.encodeWithSelector(NilRollup.ErrorInvalidPublicInputForProof.selector));
        rollup.updateState(batchIndex, oldStateRoot, newStateRoot, dataProofs, validityProof, publicDataInfoMock);

        // Stop the prank
        vm.stopPrank();
    }

    function test_updateState_toRevert_preCompileEvaluation_failure() external {
        vm.startPrank(_proposer);

        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);
        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 1;

        rollup.commitBatch(batchIndex, blobCount);

        assertTrue(rollup.isBatchCommitted(batchIndex));
        assertFalse(rollup.isBatchFinalized(batchIndex));

        bytes32 oldStateRoot = hex"8de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes32 newStateRoot = hex"9de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes[] memory dataProofs = new bytes[](1);
        dataProofs[0] =
            hex"5c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";
        bytes memory validityProof =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";

        vm.expectRevert(abi.encodeWithSelector(NilRollup.ErrorCallPointEvaluationPrecompileFailed.selector));
        rollup.updateState(batchIndex, oldStateRoot, newStateRoot, dataProofs, validityProof, publicDataInfoMock);

        vm.stopPrank();
    }

    /**
     * @notice Tests the `updateState` function to ensure it reverts when attempting to update the state with a new
     * state root that has already been finalized.
     *
     * @dev This test follows these steps:
     * 1. Starts a prank as the proposer.
     * 2. Sets a blob versioned hash for the first batch. (mock)
     * 3. Commits the first batch and verifies its committed state.
     * 4. Updates the state with the first batch's details.
     * 5. Clears the mock blob hashes.
     * 6. Sets a blob versioned hash for the second batch. (mock)
     * 7. Commits the second batch and verifies its committed state.
     * 8. Attempts to update the state with the second batch's details, expecting a revert due to the new state root
     * already being finalized.
     *
     * The test ensures that the `updateState` function correctly handles the scenario where the new state root has
     * already been finalized, preventing duplicate state updates.
     */
    function test_updateState_toRevert_with_newStateRoot_already_finalized() external {
        // Start a prank as the proposer
        vm.startPrank(_proposer);

        // Set a blob versioned hash for the first batch
        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);
        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 1;

        // Commit the first batch
        rollup.commitBatch(batchIndex, blobCount);

        // Verify the committed state of the first batch
        assertTrue(rollup.isBatchCommitted(batchIndex));
        assertFalse(rollup.isBatchFinalized(batchIndex));

        // Prepare state update details for the first batch
        bytes32 oldStateRoot = hex"8de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes32 newStateRoot = hex"9de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes[] memory dataProofs = new bytes[](1);
        dataProofs[0] =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";
        bytes memory validityProof =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";

        // Update the state with the first batch's details
        rollup.updateState(batchIndex, oldStateRoot, newStateRoot, dataProofs, validityProof, publicDataInfoMock);

        // Clear the mock blob hashes
        NilRollupMockBlob(address(rollup)).clearMockBlobHashes();

        // Set a blob versioned hash for the second batch
        batchIndex = "BATCH_2";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);

        // Commit the second batch
        rollup.commitBatch(batchIndex, blobCount);

        // Verify the committed state of the second batch
        assertTrue(rollup.isBatchCommitted(batchIndex));
        assertFalse(rollup.isBatchFinalized(batchIndex));

        // Prepare state update details for the second batch
        oldStateRoot = hex"9de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        newStateRoot = hex"9de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";

        // Expect a revert due to the new state root already being finalized
        vm.expectRevert(
            abi.encodeWithSelector(NilRollup.ErrorNewStateRootAlreadyFinalized.selector, batchIndex, newStateRoot)
        );
        rollup.updateState(batchIndex, oldStateRoot, newStateRoot, dataProofs, validityProof, publicDataInfoMock);

        // Stop the prank
        vm.stopPrank();
    }

    // Pause Tests

    function test_revert_when_commitBatch_on_paused_rollupContractProxy() external {
        vm.startPrank(_owner);
        rollup.setPause(true);
        vm.stopPrank();

        // assert that the rollup is paused
        assertTrue(PausableUpgradeable(address(rollup)).paused());

        // Start a prank as the proposer
        vm.startPrank(_proposer);

        // Set a blob versioned hash for the first batch
        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);
        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 1;

        vm.expectRevert(abi.encodeWithSelector(PausableUpgradeable.EnforcedPause.selector));
        rollup.commitBatch(batchIndex, blobCount);

        // Stop the prank
        vm.stopPrank();
    }

    function test_succeed_commitBatch_After_unPause_of_rollupContractProxy() external {
        vm.startPrank(_owner);
        rollup.setPause(true);
        vm.stopPrank();

        // assert that the rollup is paused
        assertTrue(PausableUpgradeable(address(rollup)).paused());

        vm.startPrank(_owner);
        rollup.setPause(false);
        vm.stopPrank();

        // assert that the rollup is paused
        assertFalse(PausableUpgradeable(address(rollup)).paused());

        // Start a prank as the proposer
        vm.startPrank(_proposer);

        // Set a blob versioned hash for the first batch
        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);
        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 1;

        rollup.commitBatch(batchIndex, blobCount);

        // Stop the prank
        vm.stopPrank();
    }

    // owner and admin restricted functions

    /**
     * @notice Tests the `updateState` function to ensure it can be called by both the owner and the admin.
     *
     * @dev This test follows these steps:
     * 1. Starts a prank as the owner.
     * 2. Sets a blob versioned hash for the first batch.
     * 3. Commits the first batch and verifies its committed state.
     * 4. Prepares state update details for the first batch.
     * 5. Starts a prank as the admin.
     * 6. Updates the state with the first batch's details.
     * 7. Verifies the committed and finalized state of the first batch.
     *
     * The test ensures that the `updateState` function can be called by both the owner and the admin, and that the
     * state is correctly updated and finalized.
     *
     * Background:
     * The NilRollup contract has a role hierarchy starting from the owner, then the defaultAdmin, and then the
     * proposer.
     * All actions that can be performed by the proposer can also be performed by addresses with the owner and
     * defaultAdmin roles.
     * This test verifies that the `updateState` function respects this role hierarchy, allowing both the owner and the
     * defaultAdmin to perform state updates.
     */
    function test_updateState_by_owner_and_admin() external {
        // Start a prank as the proposer
        vm.startPrank(_owner);

        // Set a blob versioned hash for the first batch
        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);
        string memory batchIndex = "BATCH_1";
        uint256 blobCount = 1;

        // Commit the first batch
        rollup.commitBatch(batchIndex, blobCount);

        vm.stopPrank();

        // Verify the committed state of the first batch
        assertTrue(rollup.isBatchCommitted(batchIndex));
        assertFalse(rollup.isBatchFinalized(batchIndex));

        // Prepare state update details for the first batch
        bytes32 oldStateRoot = hex"8de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes32 newStateRoot = hex"9de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
        bytes[] memory dataProofs = new bytes[](1);
        dataProofs[0] =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";
        bytes memory validityProof =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";

        vm.startPrank(_defaultAdmin);

        // Update the state with the first batch's details
        rollup.updateState(batchIndex, oldStateRoot, newStateRoot, dataProofs, validityProof, publicDataInfoMock);

        vm.stopPrank();

        // Verify the committed state of the second batch
        assertTrue(rollup.isBatchCommitted(batchIndex));
        assertTrue(rollup.isBatchFinalized(batchIndex));

        // Stop the prank
        vm.stopPrank();
    }
}
