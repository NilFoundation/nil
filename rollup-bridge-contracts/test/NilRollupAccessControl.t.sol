// SPDX-License-Identifier: MIT
pragma solidity 0.8.27;

import { stdJson } from "forge-std/Test.sol";
import { IAccessControl } from "@openzeppelin/contracts/access/IAccessControl.sol";
import { PausableUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import { BaseTest } from "./BaseTest.sol";
import { NilRollup } from "../contracts/NilRollup.sol";
import { NilAccessControl } from "../contracts/NilAccessControl.sol";
import { INilAccessControl } from "../contracts/interfaces/INilAccessControl.sol";
import { NilRollupMockBlob } from "./mocks/NilRollupMockBlob.sol";
import { NilRollupMockBlobInvalidScenario } from "./mocks/NilRollupMockBlobInvalidScenario.sol";
import { ITransparentUpgradeableProxy } from "@openzeppelin/contracts/proxy/transparent/TransparentUpgradeableProxy.sol";
import "forge-std/console.sol";

contract NilRollupAccessControlTest is BaseTest {
    using stdJson for string;

    bytes32 internal constant OWNER_ROLE = keccak256("OWNER_ROLE");
    bytes32 internal constant PROPOSER_ROLE_ADMIN = keccak256("PROPOSER_ROLE_ADMIN");
    bytes32 internal constant PROPOSER_ROLE = keccak256("PROPOSER_ROLE");
    bytes32 internal constant DEFAULT_ADMIN_ROLE = 0x00;

    string internal constant BATCH_ID = "BATCH_1";

    INilAccessControl public nilAccessControlInstance;

    address internal _proposerAdmin;

    address internal _proposer2;

    address internal _admin_2;

    bytes32 internal oldStateRoot = hex"8de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
    bytes32 internal newStateRoot = hex"9de4b8e9649321f6aa403b03144f068e52db6cd0b6645fc572d6a9c600f5cb91";
    bytes internal validityProof =
        hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";
    bytes[] internal dataProofs;

    function setUp() public override {
        super.setUp();
        nilAccessControlInstance = INilAccessControl(address(rollup));
        _proposerAdmin = vm.addr(10);
        _proposer2 = vm.addr(11);
        _admin_2 = vm.addr(12);

        // Set a valid versioned hash for the first batch
        bytes32 versionedHash = hex"01b8c86fa666387a77359ce7a28279db2e55c1e06772828ae65f26722b704862";
        NilRollupMockBlob(address(rollup)).setBlobVersionedHash(0, versionedHash);

        dataProofs = new bytes[](1);
        dataProofs[0] =
            hex"4c746babf097541f290a0b3bd300fa5e7874cecac18404287093b343f86eec75292693c83af3e79058a8f6a555ac92492e8b24cfdcb9b74148c0fc10917430308020c2fcb81a761c74b62042e6331d4f158702e087a32c56479e97ce611770f162606d64f90eb197b8475565ee0a37128a532ea99af9fb72673e37139eed42f60d79c671097d0b566638cc8861fd7cb66ccbecb436c53877e2e74f7db03280a7";
    }

    function prepareProposerAdmin() internal {
        // Start a prank as the default admin
        vm.startPrank(_defaultAdmin);

        // Grant the PROPOSER_ADMIN role to a new proposer admin address
        nilAccessControlInstance.grantProposerAdminRole(_proposerAdmin);

        vm.stopPrank();

        // Verify that the new proposer admin address has the PROPOSER_ADMIN role
        assertTrue(IAccessControl(rollup).hasRole(PROPOSER_ROLE_ADMIN, _proposerAdmin));
    }

    function revokeProposerAdmin() internal {
        // Start a prank as the default admin
        vm.startPrank(_defaultAdmin);

        // revoke the PROPOSER_ADMIN role to a new proposer admin address
        nilAccessControlInstance.revokeProposerAdminRole(_proposerAdmin);

        vm.stopPrank();

        // Verify that the new proposer admin address has the PROPOSER_ADMIN role
        assertFalse(IAccessControl(rollup).hasRole(PROPOSER_ROLE_ADMIN, _proposerAdmin));
    }

    function prepareProposer() internal {
        // Start a prank as the new proposer admin
        vm.startPrank(_proposerAdmin);

        // Grant the PROPOSER role to a new proposer address
        nilAccessControlInstance.grantProposerAccess(_proposer2);

        vm.stopPrank();

        console.log("prepareProposer - ProposerRole is: ");
        console.logBytes32(PROPOSER_ROLE);

        // Verify that the new proposer address has the PROPOSER role
        assertTrue(IAccessControl(rollup).hasRole(PROPOSER_ROLE, _proposer2));

        address[] memory proposers = nilAccessControlInstance.getAllProposers();

        assertEq(proposers.length, 4);
        assertEq(proposers[0], _owner);
        assertEq(proposers[1], _defaultAdmin);
        assertEq(proposers[2], _proposer);
        assertEq(proposers[3], _proposer2);
    }

    function revokeProposer() internal {
        // Start a prank as the new proposer admin
        vm.startPrank(_proposerAdmin);

        // Grant the PROPOSER role to a new proposer address
        nilAccessControlInstance.revokeProposerAccess(_proposer2);

        vm.stopPrank();

        // Verify that the new proposer address has the PROPOSER role
        assertFalse(IAccessControl(rollup).hasRole(PROPOSER_ROLE, _proposer2));

        address[] memory proposers = nilAccessControlInstance.getAllProposers();

        assertEq(proposers.length, 3);
        assertEq(proposers[0], _owner);
        assertEq(proposers[1], _defaultAdmin);
        assertEq(proposers[2], _proposer);
    }

    function execute_commit_batch() internal {
        vm.startPrank(_proposer2);
        rollup.commitBatch(BATCH_ID, 1);
        vm.stopPrank();
    }

    function execute_update_state() internal {
        vm.startPrank(_proposer2);
        rollup.updateState(BATCH_ID, oldStateRoot, newStateRoot, dataProofs, validityProof, publicDataInfoMock);
        vm.stopPrank();
    }

    function test_grant_proposer_access() public {
        prepareProposerAdmin();
        prepareProposer();
    }

    /**
     * @notice Tests the end-to-end scenario for granting and using the PROPOSER_ADMIN and PROPOSER roles.
     *
     * @dev This test follows these steps:
     * 1. Starts a prank as the default admin.
     * 2. Grants the PROPOSER_ADMIN role to a new proposer admin address.
     * 3. Verifies that the new proposer admin address has the PROPOSER_ADMIN role.
     * 4. Starts a prank as the new proposer admin.
     * 5. Grants the PROPOSER role to a new proposer address.
     * 6. Verifies that the new proposer address has the PROPOSER role.
     * 7. Starts a prank as the new proposer.
     * 8. Commits a batch with test data and updates the state with the committed batch data.
     * 9. Verifies that the batch is committed and the state is updated.
     *
     * Context:
     * The NilRollup contract has an access control hierarchy with the following roles:
     * - DEFAULT_ADMIN: The highest role, which can grant and revoke the PROPOSER_ADMIN role.
     * - PROPOSER_ADMIN: A role that can grant and revoke the PROPOSER role.
     * - PROPOSER: A role that can commit batches and update the state.
     *
     * This test showcases the end-to-end scenario where:
     * - The default admin grants the PROPOSER_ADMIN role to a new proposer admin address.
     * - The new proposer admin grants the PROPOSER role to a new proposer address.
     * - The new proposer commits a batch and updates the state.
     *
     * The test ensures that the access control hierarchy is correctly implemented and that roles can be granted and
     * used as expected.
     */
    function test_grant_proposer_admin_role_e2e() public {
        prepareProposerAdmin();
        prepareProposer();

        // Commit a batch with test data and update the state with the committed batch data
        execute_commit_batch();
        execute_update_state();
    }

    function test_revoke_proposer_admin_role() public {
        prepareProposerAdmin();
        revokeProposerAdmin();

        vm.startPrank(_proposerAdmin);
        vm.expectRevert(
            abi.encodeWithSelector(
                IAccessControl.AccessControlUnauthorizedAccount.selector, _proposerAdmin, PROPOSER_ROLE_ADMIN
            )
        );
        nilAccessControlInstance.grantProposerAccess(_proposer2);

        vm.stopPrank();
    }

    function test_revoke_proposer_role_e2e() public {
        prepareProposerAdmin();
        prepareProposer();
        revokeProposer();

        vm.expectRevert(NilAccessControl.ErrorCallerIsNotProposer.selector);
        execute_commit_batch();
    }

    // defaultAdmin access management

    function test_grant_defaultAdmin_access() public {
        vm.startPrank(_owner);

        nilAccessControlInstance.addAdmin(_admin_2);

        vm.stopPrank();

        assertTrue(IAccessControl(address(nilAccessControlInstance)).hasRole(DEFAULT_ADMIN_ROLE, _admin_2));

        // query the addresses with DefaultAdmin role
        address[] memory admins = nilAccessControlInstance.getAllAdmins();

        assertEq(admins.length, 2);
    }

    function test_transfer_ownership() public {
        address newOwner = vm.addr(100);

        vm.startPrank(_owner);

        rollup.transferOwnership(newOwner);

        vm.stopPrank();

        assertEq(newOwner, rollup.pendingOwner());
        assertEq(_owner, rollup.owner());

        assertTrue(rollup.hasRole(OWNER_ROLE, _owner));
        assertFalse(rollup.hasRole(OWNER_ROLE, newOwner));

        vm.startPrank(newOwner);

        rollup.acceptOwnership();

        vm.stopPrank();

        assertFalse(rollup.hasRole(OWNER_ROLE, _owner));
        assertTrue(rollup.hasRole(OWNER_ROLE, newOwner));
    }
}
