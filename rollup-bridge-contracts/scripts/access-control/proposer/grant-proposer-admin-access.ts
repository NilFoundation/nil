import { Contract } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadL1NetworkConfig,
    isValidAddress,
} from '../../../deploy/config/config-helper';
import { getAllProposerAdmins } from './get-all-proposer-admins';

// Load the ABI from the JSON file
const abiPath = path.join(
    __dirname,
    '../../artifacts/contracts/interfaces/INilAccessControl.sol/INilAccessControl.json',
);
const abi = JSON.parse(fs.readFileSync(abiPath, 'utf8')).abi;

// npx hardhat run scripts/access-control/proposer/grant-proposer-admin-access.ts --network sepolia
// Function to grant proposerAdmin access
export async function grantProposerAdminAccess(proposerAdminAddress: string) {
    // Lazy import inside the function
    // @ts-ignore
    const { ethers, network } = await import('hardhat');
    const networkName = network.name;
    const config = loadL1NetworkConfig(networkName);

    if (!isValidAddress(config.nilRollup.nilRollupContracts.nilRollupProxy)) {
        throw new Error('Invalid nilRollupProxy address in config');
    }

    const [signer] = await ethers.getSigners();

    const nilRollupInstance = new ethers.Contract(
        config.nilRollup.nilRollupContracts.nilRollupProxy,
        abi,
        signer,
    ) as Contract;

    const tx =
        await nilRollupInstance.grantProposerAdminRole(proposerAdminAddress);
    await tx.wait();

    console.log(`Proposer-admin access granted to ${proposerAdminAddress}`);
}

// Main function to call the grantProposerAdminAccess function
async function main() {
    const proposerAdminAddress = '';
    await grantProposerAdminAccess(proposerAdminAddress);
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
