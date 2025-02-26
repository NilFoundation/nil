import { ethers, network } from 'hardhat';
import { Contract } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadConfig,
    isValidAddress,
} from '../../../deploy/config/config-helper';
import { getRoleMembers } from '../get-role-members';
import { DEFAULT_ADMIN_ROLE } from '../../utils/roles';

const abiPath = path.join(
    __dirname,
    '../../../artifacts/contracts/interfaces/INilAccessControl.sol/INilAccessControl.json',
);
const abi = JSON.parse(fs.readFileSync(abiPath, 'utf8')).abi;

// execution instruction: npx hardhat run scripts/access-control/admin/grant-admin-access.ts --network sepolia
// Function to grant Admin access
export async function grantAdminAccess(adminAddress: string) {
    const networkName = network.name;
    const config = loadConfig(networkName);

    // Validate configuration parameters
    if (!isValidAddress(config.nilRollupProxy)) {
        throw new Error('Invalid nilRollupProxy address in config');
    }

    // Get the signer (default account)
    const [signer] = await ethers.getSigners();

    // Create a contract instance
    const nilRollupInstance = new ethers.Contract(
        config.nilRollupProxy,
        abi,
        signer,
    ) as Contract;

    // Grant proposer access
    const tx = await nilRollupInstance.addAdmin(adminAddress);
    await tx.wait();

    const admins = await getRoleMembers(DEFAULT_ADMIN_ROLE);
}

// Main function to call the grantAdminAccess function
async function main() {
    const adminAddress = '0x7A2f4530b5901AD1547AE892Bafe54c5201D1206';
    await grantAdminAccess(adminAddress);
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
