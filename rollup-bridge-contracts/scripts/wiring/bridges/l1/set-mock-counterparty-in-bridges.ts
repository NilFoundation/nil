import { ethers, network } from 'hardhat';
import { Contract } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadL1NetworkConfig,
    isValidAddress,
    loadL1MockConfig,
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

// npx hardhat run scripts/wiring/bridges/l1/set-mock-counterparty-in-bridges.ts --network geth
export async function setMockCounterpartyInBridges(networkName: string) {
    const config = loadL1NetworkConfig(networkName);
    const l1MockConfig = loadL1MockConfig(networkName);

    if (!isValidAddress(config.l1ERC20Bridge.l1ERC20BridgeProxy)) {
        throw new Error('Invalid l1ERC20BridgeProxy address in config');
    }

    if (!isValidAddress(config.l1ETHBridge.l1ETHBridgeProxy)) {
        throw new Error('Invalid l1ETHBridgeProxy address in config');
    }

    if (!isValidAddress(l1MockConfig.mockL2Bridge)) {
        throw new Error('Invalid mockL2Bridge address in config');
    }

    const [signer] = await ethers.getSigners();

    const l1ERC20BridgeInstance = new ethers.Contract(
        config.l1ERC20Bridge.l1ERC20BridgeProxy,
        l1ERC20BridgeABI,
        signer,
    ) as Contract;

    const tx = await l1ERC20BridgeInstance.setCounterpartyBridge(l1MockConfig.mockL2Bridge);
    await tx.wait();

    const counterparty_in_erc20_bridge = await l1ERC20BridgeInstance.counterpartyBridge();
    console.log(`counterparty set in erc20_bridge is: ${counterparty_in_erc20_bridge}`);

    const l1ETHBridgeInstance = new ethers.Contract(
        config.l1ETHBridge.l1ETHBridgeProxy,
        l1EthBridgeABI,
        signer,
    ) as Contract;

    const tx2 = await l1ETHBridgeInstance.setCounterpartyBridge(l1MockConfig.mockL2Bridge);
    await tx2.wait();

    const counterparty_in_eth_bridge = await l1ETHBridgeInstance.counterpartyBridge();
    console.log(`counterparty set in eth_bridge is: ${counterparty_in_eth_bridge}`);
}
