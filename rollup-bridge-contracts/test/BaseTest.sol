// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { DSTestPlus } from "solmate/test/utils/DSTestPlus.sol";
import { stdJson } from "forge-std/Test.sol";
import { Test } from "forge-std/Test.sol";
import { Vm } from "forge-std/Vm.sol";
import { ProxyAdmin } from "@openzeppelin/contracts/proxy/transparent/ProxyAdmin.sol";
import { ITransparentUpgradeableProxy } from "@openzeppelin/contracts/proxy/transparent/TransparentUpgradeableProxy.sol";
import { CustomTransparentUpgradeableProxy } from "./misc/CustomTransparentUpgradeableProxy.sol";

import { NilRollup } from "../contracts/NilRollup.sol";
import { INilRollup } from "../contracts/interfaces/INilRollup.sol";
import { EmptyContract } from "./misc/EmptyContract.sol";
import { NilVerifier } from "../contracts/verifier/NilVerifier.sol";
import { NilRollupMockBlob } from "./mocks/NilRollupMockBlob.sol";
import { NilRollupMockBlobInvalidScenario } from "./mocks/NilRollupMockBlobInvalidScenario.sol";
import "forge-std/console.sol";

// solhint-disable no-inline-assembly
contract BaseTest is Test {
    bytes32 public constant ZERO_STATE_ROOT = bytes32(0);

    using stdJson for string;

    /**
     * @notice Struct representing a single batch data item.
     * @param batchId The ID of the batch.
     * @param blobCount The number of blobs in the batch.
     * @param l2Tol1Root The root of the merkleTree with failed Deposits and l2-withdrawals.
     * @param dataProofs The data proofs for the batch.
     * @param newStateRoot The new state root after processing the batch.
     * @param oldStateRoot The old state root before processing the batch.
     * @param validityProof The validity proof for the batch.
     * @param versionedHashes The versioned hashes for the batch.
     */
    struct BatchDataItem {
        string batchId;
        uint256 blobCount;
        bytes[] dataProofs;
        bytes32 newStateRoot;
        bytes32 oldStateRoot;
        bytes validityProof;
        bytes32 l2Tol1Root;
        bytes32[] versionedHashes;
    }

    /**
     * @notice Struct representing batch data.
     * @param batches An array of batch data items.
     */
    struct BatchData {
        BatchDataItem[] batches;
    }

    /**
     * @notice Struct representing a single batch information item.
     * @param batchId The ID of the batch.
     * @param blobCount The number of blobs in the batch.
     * @param l2Tol1Root The root of the merkleTree with failed Deposits and l2-withdrawals.
     * @param dataProofs The data proofs for the batch as strings.
     * @param newStateRoot The new state root after processing the batch as a string.
     * @param oldStateRoot The old state root before processing the batch as a string.
     * @param validityProof The validity proof for the batch as a string.
     * @param versionedHashes The versioned hashes for the batch as strings.
     */
    struct BatchInfoItem {
        string batchId;
        uint256 blobCount;
        string[] dataProofs;
        string newStateRoot;
        string oldStateRoot;
        string validityProof;
        string l2Tol1Root;
        string[] versionedHashes;
    }

    /**
     * @notice Struct representing batch information.
     * @param batches An array of batch information items.
     */
    struct BatchInfo {
        BatchInfoItem[] batches;
    }

    /// @notice Error indicating invalid initialization.
    error InvalidInitialization();

    /// @notice Event emitted when a batch is committed.
    /// @param batchIndex The index of the committed batch.
    event BatchCommitted(string indexed batchIndex);

    /// @notice ProxyAdmin contract for managing the proxy.
    ProxyAdmin public proxyAdmin;

    /// @notice Address of the ProxyAdmin contract.
    address public proxyAdminAddress;

    /// @notice Placeholder contract used during proxy deployment.
    EmptyContract public placeholder;

    /// @notice Instance of the NilRollup contract.
    NilRollup public rollup;

    /// @notice Instance of the NilVerifier contract.
    NilVerifier public nilVerifier;

    /// @notice Address of the contract owner.
    address public _owner;

    /// @notice Address of the proposer.
    address public _proposer;

    /// @notice Address of the default admin.
    address public _defaultAdmin;

    /// @notice Chain ID of the L2 (NilChain).
    uint64 public _l2ChainId = 0;

    /// @notice Genesis state root hash.
    bytes32 public _genesisStateRoot = keccak256("genesisStateRoot");

    /// @notice Mock public data info used for testing.
    INilRollup.PublicDataInfo public publicDataInfoMock;

    /// @notice Placeholder data for testing.
    bytes constant PLACEHOLDER1 =
        hex"5c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";

    /// @notice Placeholder data for testing.
    bytes constant PLACEHOLDER2 =
        hex"6c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";

    /// @notice Instance of the NilRollupMockBlob contract used for testing.
    NilRollupMockBlob public nilRollupMockBlob;

    /// @notice Instance of the NilRollupMockBlobInvalidScenario contract used for testing invalid scenarios.
    NilRollupMockBlobInvalidScenario public nilRollupMockBlobInvalidScenario;

    /**
     * @notice Sets up the test environment by deploying and configuring the NilRollup contract and related components.
     *
     * @dev This function performs the following steps:
     * 1. Creates dummy addresses for the owner, default admin, and proposer.
     * 2. Initializes the public data info mock with placeholder data.
     * 3. Deploys the EmptyContract as a placeholder.
     * 4. Deploys the NilRollup contract using a proxy.
     * 5. Initializes the NilRollup contract with dummy parameters.
     * 6. Asserts that the owner and L2 chain ID are correctly set.
     * 7. Retrieves the proxy admin address and initializes the ProxyAdmin contract.
     * 8. Upgrades the NilRollup implementation to NilRollupMockBlob.
     */
    function setUp() public virtual {
        // Create dummy addresses using Foundry's hevm.addr function
        _owner = vm.addr(1);
        _defaultAdmin = vm.addr(2);
        _proposer = vm.addr(3);

        publicDataInfoMock =
            INilRollup.PublicDataInfo({ l2Tol1Root: ZERO_STATE_ROOT, messageCount: 0, l1MessageHash: ZERO_STATE_ROOT });

        placeholder = new EmptyContract();
        address proxyAddress = _deployProxy(address(0));
        rollup = NilRollup(proxyAddress);
        nilVerifier = new NilVerifier();
        nilRollupMockBlob = new NilRollupMockBlob();
        nilRollupMockBlobInvalidScenario = new NilRollupMockBlobInvalidScenario();

        vm.startPrank(_owner);

        // Initialize the contract with dummy parameters
        rollup.initialize(_l2ChainId, _owner, _defaultAdmin, address(nilVerifier), _proposer, _genesisStateRoot);

        vm.stopPrank();
        assertEq(_owner, rollup.owner());
        assertEq(rollup.l2ChainId(), 0);
        proxyAdminAddress = CustomTransparentUpgradeableProxy(payable(proxyAddress)).getAdmin();

        proxyAdmin = ProxyAdmin(proxyAdminAddress);

        // TODO fix the issue where upgradeAndCall is failing when proxy is deployed with NilRollup and later upgrade
        // fails
        vm.startPrank(_owner);
        //Upgrade the NilRollup implementation and initialize
        proxyAdmin.upgradeAndCall(
            ITransparentUpgradeableProxy(address(rollup)), address(nilRollupMockBlob), new bytes(0)
        );
        vm.stopPrank();
    }

    /**
     * @notice Updates the state with test data.
     *
     * @dev This function performs the following steps:
     * 1. Iterates over the batches in the provided batch data.
     * 2. Starts a prank as the proposer.
     * 3. Calls the `updateState` function with the batch data.
     * 4. Stops the prank.
     *
     * @param batchData The batch data to be used for updating the state.
     */
    function updateStateWithTestData(address proposerAddress, BatchData memory batchData) public {
        for (uint256 i = 0; i < batchData.batches.length; i++) {
            BatchDataItem memory batchDataItem = batchData.batches[i];

            string memory batchIndex = batchDataItem.batchId;

            vm.startPrank(proposerAddress);

            INilRollup.PublicDataInfo memory publicDataInfo = INilRollup.PublicDataInfo({
                l2Tol1Root: ZERO_STATE_ROOT,
                messageCount: 0,
                l1MessageHash: ZERO_STATE_ROOT
            });

            rollup.updateState(
                batchIndex,
                batchDataItem.oldStateRoot,
                batchDataItem.newStateRoot,
                batchDataItem.dataProofs,
                batchDataItem.validityProof,
                publicDataInfo
            );

            vm.stopPrank();
        }
    }

    /**
     * @notice Commits a batch with test data.
     *
     * @dev This function performs the following steps:
     * 1. accepts batch data
     * 2. Iterates over the batches in the generated batch data.
     * 3. Prepares mock data for the rollupMock contract.
     * 4. Starts a prank as the proposer.
     * 5. Expects the `BatchCommitted` event.
     * 6. Calls the `commitBatch` function with the batch data.
     * 7. Stops the prank.
     * 8. Asserts that the batch is committed and not finalized.
     * 9. Clears the mock blob hashes.
     */
    function commitBatchWithTestData(address proposerAddress, BatchData memory batchData) public {
        for (uint256 j = 0; j < batchData.batches.length; j++) {
            string memory batchIndex = batchData.batches[j].batchId;
            uint256 blobCount = batchData.batches[j].blobCount;

            // Prepare mock data for rollupMock contract
            for (uint256 i = 0; i < blobCount; i++) {
                NilRollupMockBlob(address(rollup)).setBlobVersionedHash(i, batchData.batches[j].versionedHashes[i]);
            }

            vm.startPrank(proposerAddress);

            // Expect the BatchCommitted event with the hashed batchIndex
            vm.expectEmit(false, false, false, true);
            emit BatchCommitted(batchIndex);

            rollup.commitBatch(batchIndex, blobCount);

            vm.stopPrank();

            assertTrue(rollup.isBatchCommitted(batchIndex));
            assertFalse(rollup.isBatchFinalized(batchIndex));

            bytes32[] memory versionedHashesOfCommittedBatch = rollup.getBlobVersionedHashes(batchIndex);

            for (uint256 i = 0; i < versionedHashesOfCommittedBatch.length; i++) {
                assertEq(versionedHashesOfCommittedBatch[i], batchData.batches[j].versionedHashes[i]);
            }

            NilRollupMockBlob(address(rollup)).clearMockBlobHashes();
        }
    }

    /**
     * @notice Converts a hexadecimal string into a bytes array.
     *
     * @param s The hexadecimal string to be converted.
     * @return r The converted bytes array.
     */
    function hexStringToBytes(string memory s) internal pure returns (bytes memory) {
        bytes memory ss = bytes(s);
        require(ss.length % 2 == 0, "Hex string length must be even");
        bytes memory r = new bytes(ss.length / 2);
        for (uint256 i = 0; i < ss.length / 2; ++i) {
            r[i] = bytes1(fromHexChar(uint8(ss[2 * i])) * 16 + fromHexChar(uint8(ss[2 * i + 1])));
        }
        return r;
    }

    /**
     * @notice Converts a hexadecimal character into its decimal value.
     *
     * @param c The hexadecimal character to be converted.
     * @return The decimal value of the hexadecimal character.
     */
    function fromHexChar(uint8 c) internal pure returns (uint8) {
        if (bytes1(c) >= bytes1("0") && bytes1(c) <= bytes1("9")) {
            return c - uint8(bytes1("0"));
        }
        if (bytes1(c) >= bytes1("a") && bytes1(c) <= bytes1("f")) {
            return 10 + c - uint8(bytes1("a"));
        }
        if (bytes1(c) >= bytes1("A") && bytes1(c) <= bytes1("F")) {
            return 10 + c - uint8(bytes1("A"));
        }
        revert("Invalid hex character");
    }

    /**
     * @notice Converts a bytes32 value into a hexadecimal string.
     *
     * @param _bytes32 The bytes32 value to be converted.
     * @return The converted hexadecimal string.
     */
    function bytes32ToHexString(bytes32 _bytes32) public pure returns (string memory) {
        bytes memory hexChars = "0123456789abcdef";
        bytes memory str = new bytes(64);
        for (uint256 i = 0; i < 32; i++) {
            str[i * 2] = hexChars[uint8(_bytes32[i] >> 4)];
            str[1 + i * 2] = hexChars[uint8(_bytes32[i] & 0x0f)];
        }
        return string(abi.encodePacked("0x", str));
    }

    /**
     * @notice Deploys a proxy contract with the specified logic contract.
     *
     * @param _logic The address of the logic contract.
     * @return The address of the deployed proxy contract.
     */
    function _deployProxy(address _logic) internal returns (address) {
        if (_logic == address(0)) _logic = address(new NilRollup());
        CustomTransparentUpgradeableProxy proxy = new CustomTransparentUpgradeableProxy(_logic, _owner, new bytes(0));
        return address(proxy);
    }
}
