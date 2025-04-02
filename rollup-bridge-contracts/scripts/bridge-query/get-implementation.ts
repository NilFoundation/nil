import { ethers, network } from 'hardhat';
import { Contract } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadL1NetworkConfig,
    isValidAddress,
} from '../../../../deploy/config/config-helper';
const abiPath = path.join(
    __dirname,
    '../../../../artifacts/contracts/bridge/l1/interfaces/IL1BridgeMessenger.sol/IL1BridgeMessenger.json',
);
const abi = JSON.parse(fs.readFileSync(abiPath, 'utf8')).abi;

// npx hardhat run scripts/wiring/bridges/l1/get-implementation.ts --network geth
export async function getImplementationAddress() {
    const networkName = network.name;
    const config = loadL1NetworkConfig(networkName);
    const provider = ethers.provider;

    if (!isValidAddress(config.l1BridgeMessengerConfig.l1BridgeMessengerProxy)) {
        throw new Error('Invalid l1BridgeMessengerProxy address in config');
    }

    const l1BridgeMessengerInstance = new ethers.Contract(
        config.l1BridgeMessengerConfig.l1BridgeMessengerProxy,
        abi,
        provider
    ) as Contract;


    const implementationAddress = await l1BridgeMessengerInstance.getImplementation();

    console.log(`implementationAddress for proxy is: ${implementationAddress}`);
}

async function main() {
    await getImplementationAddress();
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
