import { Contract } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadL1NetworkConfig,
    isValidAddress,
} from '../../../deploy/config/config-helper';
import { getAllProposers } from './get-all-proposers';
import { isAProposer } from './is-a-proposer';

const abiPath = path.join(
    __dirname,
    '../../../artifacts/contracts/interfaces/INilAccessControl.sol/INilAccessControl.json',
);
const abi = JSON.parse(fs.readFileSync(abiPath, 'utf8')).abi;

// npx hardhat run scripts/access-control/proposer/revoke-proposer-access.ts --network sepolia
export async function revokeProposerAccess(proposerAddress: string) {
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

    let isAProposerResponse: Boolean = await isAProposer(proposerAddress);

    if (!isAProposerResponse) {
        throw new Error(
            `account: ${proposerAddress} is not a proposer. so revokeProposer cannot be initiated`,
        );
    }

    const tx = await nilRollupInstance.revokeProposerAccess(proposerAddress);
    await tx.wait();

    isAProposerResponse = await isAProposer(proposerAddress);

    if (isAProposerResponse) {
        throw new Error(
            `revokeProposer failed. account: ${proposerAddress} is still a proposer.`,
        );
    }
}

// Main function to call the revokeProposerAccess function
async function main() {
    const proposerAddress = '';
    await revokeProposerAccess(proposerAddress);
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
