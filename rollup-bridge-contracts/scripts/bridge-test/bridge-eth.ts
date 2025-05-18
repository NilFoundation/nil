import { ethers, network } from 'hardhat';
import { Contract, TransactionReceipt } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadL1NetworkConfig,
    isValidAddress,
    loadNilNetworkConfig,
    L2NetworkConfig,
    L1NetworkConfig,
} from '../../deploy/config/config-helper';
import { bigIntReplacer, extractAndParseMessageSentEventLog, MessageSentEvent } from './get-messenger-events';

const l1EthBridgeABIPath = path.join(
    __dirname,
    '../../artifacts/contracts/bridge/l1/interfaces/IL1ETHBridge.sol/IL1ETHBridge.json',
);
const l1EthBridgeABI = JSON.parse(fs.readFileSync(l1EthBridgeABIPath, 'utf8')).abi;

// npx hardhat run scripts/bridge-test/bridge-eth.ts --network geth
export async function bridgeETH() {
    const networkName = network.name;
    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);

    if (!isValidAddress(config.l1ETHBridge.l1ETHBridgeProxy)) {
        throw new Error('Invalid l1ETHBridgeProxy address in config');
    }

    const signers = await ethers.getSigners();

    const signer = signers[0]; // The main signer

    const signerAddress = signer.address;
    const l1ETHBridgeInstance = new ethers.Contract(
        config.l1ETHBridge.l1ETHBridgeProxy,
        l1EthBridgeABI,
        signer,
    ) as Contract;


    // save the nilMessageTree Address in the json config for l2
    const l2NetworkConfig: L2NetworkConfig = loadNilNetworkConfig("local");

    const l2DepositRecipient = config.l1TestConfig.l2DepositRecipient;
    const l2FeeRefundAddress = config.l1TestConfig.l2FeeRefundRecipient;
    const eth_amount = config.l1TestConfig.l1ETHDepositTestConfig.amount;
    const gasLimit = config.l1TestConfig.l1ETHDepositTestConfig.gasLimit;
    const total_native_amount = config.l1TestConfig.l1ETHDepositTestConfig.totalNativeAmount;
    const userMaxFeePerGas = config.l1TestConfig.l1ETHDepositTestConfig.userMaxFeePerGas;
    const userMaxPriorityFeePerGas = config.l1TestConfig.l1ETHDepositTestConfig.userMaxPriorityFeePerGas;

    console.log(`bridging ${eth_amount} (WEI) to recipient: ${l2DepositRecipient}`);

    const tx = await l1ETHBridgeInstance.depositETH(eth_amount, l2DepositRecipient, l2FeeRefundAddress, gasLimit, userMaxFeePerGas, userMaxPriorityFeePerGas, { value: total_native_amount });
    await tx.wait();

    const transactionHash = tx.hash;

    console.log(`transactionHash for ETHDeposit is: ${transactionHash}`);

    const transactionDetails: TransactionReceipt = await ethers.provider.getTransactionReceipt(transactionHash);
    if (!transactionDetails || transactionDetails.status == 0) {
        throw new Error(`DepositETH L1Bridge transaction failed`);
    } else {
        console.log(`Successful DepositETH transaction on L1ETHBridge`);
    }

    console.log(`transactionDetails are: ${JSON.stringify(transactionDetails)}`);

    console.log(`DepositETH via L1ETHBridge costed -> cumlativeGasUsed : ${transactionDetails.cumulativeGasUsed} - gasUsed: ${transactionDetails.gasUsed}`);

    const messageSentEventLogData = await extractAndParseMessageSentEventLog(transactionHash);

    if (!messageSentEventLogData) {
        throw new Error(`Failed to parse MessageSent event Log emitted by L1BridgeMessenger contract`);
    }

    const messageSentEvent: MessageSentEvent = messageSentEventLogData;

    console.log(`messageSentEvent for depositETH is: ${JSON.stringify(messageSentEvent, bigIntReplacer, 2)}`);

    const messageHash = messageSentEvent.messageHash;

}

async function main() {
    await bridgeETH();
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
