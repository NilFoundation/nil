import { Contract } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    isValidAddress,
    loadL1NetworkConfig,
} from '../../../deploy/config/config-helper';

const abiPath = path.join(
    __dirname,
    '../../artifacts/contracts/interfaces/INilAccessControl.sol/INilAccessControl.json',
);
const abi = JSON.parse(fs.readFileSync(abiPath, 'utf8')).abi;

export async function isAProposer(proposerAddress: string) {
    // Lazy import inside the function
    // @ts-ignore
    const { ethers, network } = await import('hardhat');

    const networkName = network.name;
    const config = loadL1NetworkConfig(networkName);

    if (!isValidAddress(config.nilRollup.nilRollupContracts.nilRollupProxy)) {
        throw new Error('Invalid nilRollupProxy address in config');
    }

    const [signer] = await ethers.getSigners();

    const nilAccessControlInstance: Contract = new ethers.Contract(
        config.nilRollup.nilRollupContracts.nilRollupProxy,
        abi,
        signer,
    ) as Contract;

    const isAProposerResponse =
        await nilAccessControlInstance.isAProposer(proposerAddress);

    const isProposer = Boolean(isAProposerResponse);

    if (isProposer) {
        console.log(
            `account: ${proposerAddress} is a Proposer on network: ${networkName} for rollupContract: ${config.nilRollupProxy}`,
        );
    } else {
        console.log(
            `account: ${proposerAddress} is not-a Proposer on network: ${networkName} for rollupContract: ${config.nilRollupProxy}`,
        );
    }

    return isProposer;
}
