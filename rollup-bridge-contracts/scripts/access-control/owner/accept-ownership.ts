import { ethers, network } from 'hardhat';
import { Contract, ZeroAddress } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadConfig,
    isValidAddress,
} from '../../../deploy/config/config-helper';
import { getRollupOwner } from './get-owner';
import { getRollupPendingOwner } from './get-pending-owner';

// Load the ABI from the JSON file
const abiPath = path.join(
    __dirname,
    '../../../artifacts/contracts/NilRollup.sol/NilRollup.json',
);
const abi = JSON.parse(fs.readFileSync(abiPath, 'utf8')).abi;

// npx hardhat run scripts/access-control/owner/accept-ownership.ts --network sepolia
export async function acceptOwnership() {
    const networkName = network.name;
    const config = loadConfig(networkName);

    // Validate configuration parameters
    if (!isValidAddress(config.nilRollupProxy)) {
        throw new Error('Invalid nilRollupProxy address in config');
    }

    // Get the signer (default account)
    const [signer] = await ethers.getSigners();

    let currentOwner = await getRollupOwner();
    let pendingOwner = await getRollupPendingOwner();

    if (pendingOwner == ZeroAddress) {
        throw new Error(
            `Pending owner (${pendingOwner}) must not a zero address`,
        );
    }

    if (pendingOwner !== signer.address) {
        throw new Error(
            `Pending-Owner (${pendingOwner}) must be the same as the signer (${signer.address})`,
        );
    }

    if (currentOwner === pendingOwner) {
        throw new Error(
            `Current owner (${currentOwner}) must not be the same as the pending owner (${pendingOwner})`,
        );
    }

    // Create a contract instance
    const nilRollupInstance = new ethers.Contract(
        config.nilRollupProxy,
        abi,
        signer,
    ) as Contract;

    // Grant proposer access
    const tx = await nilRollupInstance.acceptOwnership();

    await tx.wait();

    currentOwner = await getRollupOwner();

    console.log(
        `owner of the rollupProxy after acceptance is: ${currentOwner}`,
    );

    pendingOwner = await getRollupPendingOwner();

    if (pendingOwner == currentOwner) {
        throw new Error(
            `acceptOwnership is not successful as pendingOwner: ${pendingOwner} extracted from rollupProxy is not same as the newOwner`,
        );
    }

    if (pendingOwner != ZeroAddress) {
        throw new Error(
            `pendingOwner: ${pendingOwner} extracted from rollupProxy is non-zero-address`,
        );
    }
}

// Main function to call the acceptOwnership function
async function main() {
    await acceptOwnership();
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
