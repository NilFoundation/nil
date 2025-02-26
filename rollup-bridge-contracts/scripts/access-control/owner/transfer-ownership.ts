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

const abiPath = path.join(
    __dirname,
    '../../../artifacts/contracts/NilRollup.sol/NilRollup.json',
);
const abi = JSON.parse(fs.readFileSync(abiPath, 'utf8')).abi;

// npx hardhat run scripts/access-control/owner/transfer-ownership.ts --network sepolia
export async function transferOwnership(newOwner: string) {
    const networkName = network.name;
    const config = loadConfig(networkName);

    if (!isValidAddress(config.nilRollupProxy)) {
        throw new Error('Invalid nilRollupProxy address in config');
    }

    const [signer] = await ethers.getSigners();

    const currentOwner = await getRollupOwner();
    let pendingOwner = await getRollupPendingOwner();

    if (currentOwner !== signer.address) {
        throw new Error(
            `Current owner (${currentOwner}) must be the same as the signer (${signer.address})`,
        );
    }

    if (currentOwner === pendingOwner) {
        throw new Error(
            `Current owner (${currentOwner}) must not be the same as the pending owner (${pendingOwner})`,
        );
    }

    if (pendingOwner !== ZeroAddress) {
        throw new Error(
            `Pending owner (${pendingOwner}) must be the zero address`,
        );
    }

    const nilRollupInstance = new ethers.Contract(
        config.nilRollupProxy,
        abi,
        signer,
    ) as Contract;

    const tx = await nilRollupInstance.transferOwnership(newOwner);

    await tx.wait();

    pendingOwner = await getRollupPendingOwner();

    if (pendingOwner == ZeroAddress) {
        throw new Error(
            `pendingOwner: ${pendingOwner} extracted from rollupProxy is zero-address`,
        );
    }

    if (pendingOwner != newOwner) {
        throw new Error(
            `pendingOwner: ${pendingOwner} extracted from rollupProxy is not set as the newOwner: ${newOwner}`,
        );
    }
}

// Main function to call the transferOwnership function
async function main() {
    const newOwner = '';
    await transferOwnership(newOwner);
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
