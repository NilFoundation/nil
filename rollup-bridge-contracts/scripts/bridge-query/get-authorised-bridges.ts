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

// npx hardhat run scripts/wiring/bridges/l1/get-authorised-bridges.ts --network geth
export async function getAuthoriseBridges() {
    const networkName = network.name;
    const config = loadL1NetworkConfig(networkName);

    if (!isValidAddress(config.l1ERC20BridgeConfig.l1ERC20BridgeProxy)) {
        throw new Error('Invalid l1ERC20BridgeProxy address in config');
    }

    if (!isValidAddress(config.l1ETHBridgeConfig.l1ETHBridgeProxy)) {
        throw new Error('Invalid l1ETHBridgeProxy address in config');
    }

    if (!isValidAddress(config.l1BridgeMessengerConfig.l1BridgeMessengerProxy)) {
        throw new Error('Invalid l1BridgeMessengerProxy address in config');
    }

    const [signer] = await ethers.getSigners();

    const l1BridgeMessengerInstance = new ethers.Contract(
        config.l1BridgeMessengerConfig.l1BridgeMessengerProxy,
        abi,
        signer,
    ) as Contract;

    const authorisedBridges = await l1BridgeMessengerInstance.getAuthorizedBridges();
    console.log(`authorised-bridges in L1BridgeMessenger are: ${authorisedBridges}`);
}

async function main() {
    await getAuthoriseBridges();
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
