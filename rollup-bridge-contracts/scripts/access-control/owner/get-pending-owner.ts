import { ethers, network } from 'hardhat';
import { Contract } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadL1NetworkConfig,
    isValidAddress,
} from '../../../deploy/config/config-helper';

// Load the ABI from the JSON file
const abiPath = path.join(
    __dirname,
    '../../../artifacts/contracts/NilRollup.sol/NilRollup.json',
);
const abi = JSON.parse(fs.readFileSync(abiPath, 'utf8')).abi;

export async function getRollupPendingOwner() {
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

    const rollupProxyPendingOwner = await nilRollupInstance.pendingOwner();

    return rollupProxyPendingOwner;
}

// Main function to call the getRollupPendingOwner function for an account
async function main() {
    await getRollupPendingOwner();
}

// npx hardhat run scripts/access-control/owner/get-pending-owner.ts --network sepolia
main().catch((error) => {
    console.error(error);
    process.exit(1);
});
