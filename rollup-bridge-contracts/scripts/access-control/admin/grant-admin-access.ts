import { Contract } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import { getRoleMembers } from '../get-role-members';
import { DEFAULT_ADMIN_ROLE } from '../../utils/roles';
import { isValidAddress, loadL1NetworkConfig } from '../../../deploy/config/config-helper';

const abiPath = path.join(
    __dirname,
    '../../../artifacts/contracts/interfaces/INilAccessControl.sol/INilAccessControl.json',
);
const abi = JSON.parse(fs.readFileSync(abiPath, 'utf8')).abi;

// execution instruction: npx hardhat run scripts/access-control/admin/grant-admin-access.ts --network sepolia
// Function to grant Admin access
export async function grantAdminAccess(adminAddress: string) {
    // Lazy import inside the function
    // @ts-ignore
    const { ethers, network } = await import('hardhat');
    const networkName = network.name;
    const config = loadL1NetworkConfig(networkName);

    // Validate configuration parameters
    if (!isValidAddress(config.nilRollup.nilRollupContracts.nilRollupProxy)) {
        throw new Error('Invalid nilRollupProxy address in config');
    }

    // Get the signer (default account)
    const [signer] = await ethers.getSigners();

    // Create a contract instance
    const nilRollupInstance = new ethers.Contract(
        config.nilRollup.nilRollupContracts.nilRollupProxy,
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
