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

export async function authoriseBridges(networkName: string) {
    const config = loadL1NetworkConfig(networkName);

    if (!isValidAddress(config.l1ERC20Bridge.l1ERC20BridgeProxy)) {
        throw new Error('Invalid l1ERC20BridgeProxy address in config');
    }

    if (!isValidAddress(config.l1ETHBridge.l1ETHBridgeProxy)) {
        throw new Error('Invalid l1ETHBridgeProxy address in config');
    }

    if (!isValidAddress(config.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerProxy)) {
        throw new Error('Invalid l1BridgeMessengerProxy address in config');
    }

    const [signer] = await ethers.getSigners();

    const l1BridgeMessengerInstance = new ethers.Contract(
        config.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerProxy,
        abi,
        signer,
    ) as Contract;

    console.log(`authorising l1-bridges: ${[config.l1ERC20Bridge.l1ERC20BridgeProxy, config.l1ETHBridge.l1ETHBridgeProxy]} to engage with l1BridgeMessenger: ${config.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerProxy}`);
    const tx = await l1BridgeMessengerInstance.authorizeBridges(
        [config.l1ETHBridge.l1ETHBridgeProxy, config.l1ERC20Bridge.l1ERC20BridgeProxy]);

    await tx.wait();
}
