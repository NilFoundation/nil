import { ethers, network } from 'hardhat';
import { Contract } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadL1NetworkConfig,
    isValidAddress,
} from '../../../../deploy/config/config-helper';

const l1EthBridgeABIPath = path.join(
    __dirname,
    '../../../../artifacts/contracts/bridge/l1/interfaces/IL1ETHBridge.sol/IL1ETHBridge.json',
);
const l1EthBridgeABI = JSON.parse(fs.readFileSync(l1EthBridgeABIPath, 'utf8')).abi;

const l1ERC20BridgeABIPath = path.join(
    __dirname,
    '../../../../artifacts/contracts/bridge/l1/interfaces/IL1ERC20Bridge.sol/IL1ERC20Bridge.json',
);
const l1ERC20BridgeABI = JSON.parse(fs.readFileSync(l1ERC20BridgeABIPath, 'utf8')).abi;

// npx hardhat run scripts/wiring/bridges/l1/set-messenger-in-bridges.ts --network geth
export async function setMessengerInBridges(networkName: string) {
    const config = loadL1NetworkConfig(networkName);

    if (!isValidAddress(config.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerProxy)) {
        throw new Error('Invalid l1BridgeMessengerProxy address in config');
    }

    if (!isValidAddress(config.l1ETHBridge.l1ETHBridgeProxy)) {
        throw new Error('Invalid l1ETHBridgeProxy address in config');
    }

    if (!isValidAddress(config.l1BridgeRouter.l1BridgeRouterProxy)) {
        throw new Error('Invalid l1BridgeRouterProxy address in config');
    }

    const [signer] = await ethers.getSigners();

    const l1ERC20BridgeInstance = new ethers.Contract(
        config.l1ERC20Bridge.l1ERC20BridgeProxy,
        l1ERC20BridgeABI,
        signer,
    ) as Contract;

    const tx = await l1ERC20BridgeInstance.setMessenger(config.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerProxy);
    await tx.wait();

    const messenger_in_erc20_bridge = await l1ERC20BridgeInstance.messenger();
    console.log(`messenger set in erc20_bridge is: ${messenger_in_erc20_bridge}`);

    const l1ETHBridgeInstance = new ethers.Contract(
        config.l1ETHBridge.l1ETHBridgeProxy,
        l1EthBridgeABI,
        signer,
    ) as Contract;

    const tx2 = await l1ETHBridgeInstance.setMessenger(config.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerProxy);
    await tx2.wait();

    const messenger_in_eth_bridge = await l1ETHBridgeInstance.messenger();
    console.log(`messenger set in eth_bridge is: ${messenger_in_eth_bridge}`);
}
