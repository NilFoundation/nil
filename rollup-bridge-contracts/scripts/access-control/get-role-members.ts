import { ethers, network } from 'hardhat';
import { Contract } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import { loadConfig, isValidAddress } from '../../deploy/config/config-helper';

const abiPath = path.join(
    __dirname,
    '../../artifacts/contracts/NilAccessControl.sol/NilAccessControl.json',
);
const abi = JSON.parse(fs.readFileSync(abiPath, 'utf8')).abi;

// npx hardhat run scripts/access-control/get-role-members.ts --network sepolia
export async function getRoleMembers(roleHash: string) {
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

    const roleMembers = await nilRollupInstance.getRoleMembers(roleHash);
    return roleMembers;
}
