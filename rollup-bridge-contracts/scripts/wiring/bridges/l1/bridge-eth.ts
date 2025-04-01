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

// npx hardhat run scripts/wiring/bridges/l1/bridge-eth.ts --network geth
export async function bridgeETH() {
    const networkName = network.name;
    const config = loadL1NetworkConfig(networkName);

    if (!isValidAddress(config.l1ETHBridgeConfig.l1ETHBridgeProxy)) {
        throw new Error('Invalid l1ETHBridgeProxy address in config');
    }

    const signers = await ethers.getSigners();

    const signer = signers[0]; // The main signer

    const signerAddress = signer.address;
    console.log(`signerAddress is: ${signerAddress}`);

    const l1ETHBridgeInstance = new ethers.Contract(
        config.l1ETHBridgeConfig.l1ETHBridgeProxy,
        l1EthBridgeABI,
        signer,
    ) as Contract;

    const recipientAddress = "0x66bFaD51E02513C5B6bEfe1Acc9a31Cb6eE152F1";
    const eth_amount = 100;
    const l2FeeRefundRecipientAddress = "0x878f824Ffde85B7Bd6ad6c6Fd97275bb6724c55a";
    const gasLimit = 1000;
    const total_native_amount = 1200000000;
    const userFeePerGas = 0;
    const userMaxPriorityFeePerGas = 0;

    console.log(`bridging ${eth_amount} (WEI) to recipient: ${recipientAddress} and with l2FeeRefundRecipientAddress: ${l2FeeRefundRecipientAddress}`);

    const tx = await l1ETHBridgeInstance.depositETH(recipientAddress, eth_amount, l2FeeRefundRecipientAddress, gasLimit, userFeePerGas, userMaxPriorityFeePerGas, { value: total_native_amount });
    const transactionReceipt = await tx.wait();

    console.log(`transactionReceipt is: ${JSON.stringify(transactionReceipt)}`);

    const transactionHash = tx.hash;

    console.log(`About to get transactionDetails for transactionHash: ${transactionHash}`);

    const transactionDetails = await ethers.provider.getTransactionReceipt(transactionHash);

    console.log(`transactionDetails for hash: ${transactionHash} is: ${JSON.stringify(transactionDetails)}`);
}

async function main() {
    await bridgeETH();
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
