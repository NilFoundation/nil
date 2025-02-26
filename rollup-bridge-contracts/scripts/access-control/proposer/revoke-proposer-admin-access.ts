import { ethers, network } from 'hardhat';
import { Contract } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadConfig,
    isValidAddress,
} from '../../../deploy/config/config-helper';

const abiPath = path.join(
    __dirname,
    '../../artifacts/contracts/interfaces/INilAccessControl.sol/INilAccessControl.json',
);
const abi = JSON.parse(fs.readFileSync(abiPath, 'utf8')).abi;

// npx hardhat run scripts/access-control/proposer/revoke-proposer-admin-access.ts --network sepolia
export async function revokeProposerAdminAccess(proposerAdminAddress: string) {
    const networkName = network.name;
    const config = loadConfig(networkName);

    if (!isValidAddress(config.nilRollupProxy)) {
        throw new Error('Invalid nilRollupProxy address in config');
    }

    const [signer] = await ethers.getSigners();

    const nilRollupInstance = new ethers.Contract(
        config.nilRollupProxy,
        abi,
        signer,
    ) as Contract;

    const tx =
        await nilRollupInstance.revokeProposerAdminRole(proposerAdminAddress);
    await tx.wait();
}

async function main() {
    const proposerAdminAddress = '';
    await revokeProposerAdminAccess(proposerAdminAddress);
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
