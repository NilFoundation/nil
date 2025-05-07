import { ethers, network } from 'hardhat';
import { Contract } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadL1NetworkConfig,
    isValidAddress,
    loadNilNetworkConfig,
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

// npx hardhat run scripts/wiring/bridges/l1/set-counterparty-in-bridges.ts --network geth
export async function setCounterpartyInBridges(networkName: string) {
    const config = loadL1NetworkConfig(networkName);
    const l2Config = loadNilNetworkConfig("local");

    if (!isValidAddress(config.l1ERC20Bridge.l1ERC20BridgeProxy)) {
        throw new Error('Invalid l1ERC20BridgeProxy address in config');
    }

    if (!isValidAddress(l2Config.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.l2EnshrinedTokenBridgeProxy)) {
        throw new Error('Invalid l2EnshrinedTokenBridgeProxy address in l2Config');
    }

    if (!isValidAddress(config.l1ETHBridge.l1ETHBridgeProxy)) {
        throw new Error('Invalid l1ETHBridgeProxy address in config');
    }

    if (!isValidAddress(l2Config.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy)) {
        throw new Error('Invalid l2ETHBridgeProxy address in l2Config');
    }

    const [signer] = await ethers.getSigners();

    const l1ERC20BridgeInstance = new ethers.Contract(
        config.l1ERC20Bridge.l1ERC20BridgeProxy,
        l1ERC20BridgeABI,
        signer,
    ) as Contract;

    const tx = await l1ERC20BridgeInstance.setCounterpartyBridge(l2Config.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.l2EnshrinedTokenBridgeProxy);
    await tx.wait();

    const counterparty_in_erc20_bridge = await l1ERC20BridgeInstance.counterpartyBridge();
    if (counterparty_in_erc20_bridge != l2Config.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.l2EnshrinedTokenBridgeProxy) {
        throw Error(`Invalid counterpartyBridge: ${counterparty_in_erc20_bridge} set in L1ERC20Bridge. \n expected counterpartyBridge is: ${l2Config.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.l2EnshrinedTokenBridgeProxy}`)
    }

    console.log(`successfully set the counterpartyBridge: ${l2Config.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.l2EnshrinedTokenBridgeProxy} in l1ETHBridge: ${config.l1ERC20Bridge.l1ERC20BridgeProxy}`);

    const l1ETHBridgeInstance = new ethers.Contract(
        config.l1ETHBridge.l1ETHBridgeProxy,
        l1EthBridgeABI,
        signer,
    ) as Contract;

    const tx2 = await l1ETHBridgeInstance.setCounterpartyBridge(l2Config.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy);
    await tx2.wait();

    const counterparty_in_eth_bridge = await l1ETHBridgeInstance.counterpartyBridge();

    console.log(`successfully set the counterpartyBridge: ${l2Config.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy} in l1ETHBridge: ${config.l1ETHBridge.l1ETHBridgeProxy}`);

    if (counterparty_in_eth_bridge != l2Config.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy) {
        throw Error(`Invalid counterpartyBridge: ${counterparty_in_eth_bridge} set in L1ETHBridge. \n expected counterpartyBridge is: ${l2Config.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy}`)
    }

}

async function main() {
    const networkName = network.name;
    await setCounterpartyInBridges(networkName);
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});