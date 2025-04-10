import { ethers, network } from 'hardhat';
import { Contract, TransactionReceipt } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadL1NetworkConfig,
    isValidAddress,
} from '../../deploy/config/config-helper';
import { bigIntReplacer, extractAndParseMessageSentEventLog, MessageSentEvent } from './get-messenger-events';

const l1BridgeRouterABIPath = path.join(
    __dirname,
    '../../artifacts/contracts/bridge/l1/interfaces/IL1BridgeRouter.sol/IL1BridgeRouter.json',
);
const l1BridgeRouterABI = JSON.parse(fs.readFileSync(l1BridgeRouterABIPath, 'utf8')).abi;

// npx hardhat run scripts/bridge-test/bridge-eth-via-router.ts --network geth
export async function bridgeETHViaRouter() {
    const networkName = network.name;
    const config = loadL1NetworkConfig(networkName);

    if (!isValidAddress(config.l1ETHBridge.l1ETHBridgeProxy)) {
        throw new Error('Invalid l1ETHBridgeProxy address in config');
    }

    const signers = await ethers.getSigners();

    const signer = signers[0]; // The main signer

    const signerAddress = signer.address;
    const l1BridgeRouterInstance = new ethers.Contract(
        config.l1BridgeRouter.l1BridgeRouterProxy,
        l1BridgeRouterABI,
        signer,
    ) as Contract;

    const l2DepositRecipient = "0x66bFaD51E02513C5B6bEfe1Acc9a31Cb6eE152F1";
    const l2FeeRefundAddress = "0x878f824Ffde85B7Bd6ad6c6Fd97275bb6724c55a";
    const eth_amount = 100;
    const gasLimit = 1000;
    const total_native_amount = 1200000000;
    const userMaxFeePerGas = 0;
    const userMaxPriorityFeePerGas = 0;

    console.log(`bridging ${eth_amount} (WEI) to recipient: ${l2DepositRecipient}`);

    const tx = await l1BridgeRouterInstance.depositETH(eth_amount, l2DepositRecipient, l2FeeRefundAddress, gasLimit, userMaxFeePerGas, userMaxPriorityFeePerGas, { value: total_native_amount });
    await tx.wait();

    const transactionHash = tx.hash;

    console.log(`transactionHash for ETHDeposit is: ${transactionHash}`);

    const transactionDetails: TransactionReceipt = await ethers.provider.getTransactionReceipt(transactionHash);
    if (!transactionDetails || transactionDetails.status == 0) {
        throw new Error(`DepositETH L1Bridge transaction failed`);
    } else {
        console.log(`Successful DepositETH transaction on L1ETHBridge initiated via L1BridgeRouter`);
    }

    console.log(`DepositETH via L1BridgeRouter costed -> cumlativeGasUsed : ${transactionDetails.cumulativeGasUsed} - gasUsed: ${transactionDetails.gasUsed}`);

    const messageSentEventLogData = await extractAndParseMessageSentEventLog(transactionHash);

    if (!messageSentEventLogData) {
        throw new Error(`Failed to parse MessageSent event Log emitted by L1BridgeMessenger contract`);
    }

    const messageSentEvent: MessageSentEvent = messageSentEventLogData;

    console.log(`messageSentEvent for depositETH via Router is: ${JSON.stringify(messageSentEvent, bigIntReplacer, 2)}`);
}

async function main() {
    await bridgeETHViaRouter();
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
