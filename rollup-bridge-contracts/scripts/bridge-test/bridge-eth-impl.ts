import { Contract, TransactionReceipt, Signer } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadL1NetworkConfig,
    isValidAddress,
    loadNilNetworkConfig,
    L2NetworkConfig,
    L1NetworkConfig,
    saveNilNetworkConfig,
} from '../../deploy/config/config-helper';
import { bigIntReplacer, extractAndParseMessageSentEventLog, MessageSentEvent } from './get-messenger-events';
import { loadL2DepositRecipientSmartAccount } from "../../task/nil-smart-account";
import { Hex } from '@nilfoundation/niljs';

const l1EthBridgeABIPath = path.join(
    __dirname,
    '../../artifacts/contracts/bridge/l1/interfaces/IL1ETHBridge.sol/IL1ETHBridge.json',
);
const l1EthBridgeABI = JSON.parse(fs.readFileSync(l1EthBridgeABIPath, 'utf8')).abi;

const nilGasPriceOracleABIPath = path.join(
    __dirname,
    '../../artifacts/contracts/bridge/l1/interfaces/INilGasPriceOracle.sol/INilGasPriceOracle.json',
);
const nilGasPriceOracleABI = JSON.parse(fs.readFileSync(nilGasPriceOracleABIPath, 'utf8')).abi;

export async function bridgeETHAux(
    l1NetworkName: string = 'geth',
    source: Signer,
    l2DepositRecipient: Hex,
    weiAmount: BigInt,
) {
    // Lazy import inside the function
    // @ts-ignore
    const { ethers } = await import('hardhat');
    
    console.log(`Network name is: ${l1NetworkName}`); 
    const config: L1NetworkConfig = loadL1NetworkConfig(l1NetworkName);

    if (!isValidAddress(config.l1ETHBridge.l1ETHBridgeProxy)) {
        throw new Error('Invalid l1ETHBridgeProxy address in config');
    }

    const l1ETHBridgeInstance = new ethers.Contract(
        config.l1ETHBridge.l1ETHBridgeProxy,
        l1EthBridgeABI,
        source,
    ) as Contract;

    console.log(`Using l2DepositRecipient: ${l2DepositRecipient}`);
    console.log(`Using source: ${await source.getAddress()}`);
     
    const l2FeeRefundAddress = config.l1TestConfig.l2FeeRefundRecipient;
    const gasLimit = config.l1TestConfig.l1ETHDepositTestConfig.gasLimit;
    const total_native_amount = config.l1TestConfig.l1ETHDepositTestConfig.totalNativeAmount;
    const userMaxFeePerGas = config.l1TestConfig.l1ETHDepositTestConfig.userMaxFeePerGas;
    const userMaxPriorityFeePerGas = config.l1TestConfig.l1ETHDepositTestConfig.userMaxPriorityFeePerGas;

    console.log(`gasLimit: ${gasLimit} ${typeof gasLimit}`);
    console.log(`userMaxFeePerGas: ${userMaxFeePerGas} ${typeof userMaxFeePerGas}`);
    console.log(`userMaxPriorityFeePerGas: ${userMaxPriorityFeePerGas} ${typeof userMaxPriorityFeePerGas}`);
    
    const nilGasPriceOracleInstance = new ethers.Contract(
        config.nilGasPriceOracle.nilGasPriceOracleContracts.nilGasPriceOracleProxy,
        nilGasPriceOracleABI,
        source,
    ) as Contract;

    const feeCreditData = await nilGasPriceOracleInstance.computeFeeCredit(
        gasLimit,
        userMaxFeePerGas,
        userMaxPriorityFeePerGas
    );

    // Log the parsed FeeCreditData struct
    console.log("Parsed FeeCreditData:");
    console.log(`Nil Gas Limit: ${feeCreditData.nilGasLimit.toString()}`);
    console.log(`Max Fee Per Gas: ${feeCreditData.maxFeePerGas.toString()}`);
    console.log(`Max Priority Fee Per Gas: ${feeCreditData.maxPriorityFeePerGas.toString()}`);
    console.log(`Fee Credit: ${feeCreditData.feeCredit.toString()}`);


    // Use the feeCredit value in your calculations
    const totalNativeAmount = weiAmount + feeCreditData.feeCredit;
    console.log(`totalNativeAmoutn computed to be used in bridge is: ${totalNativeAmount}`);

    // Log all test input parameters
    console.log("Test Input Parameters:");
    console.log(`L2 Deposit Recipient: ${l2DepositRecipient}`);
    console.log(`L2 Fee Refund Address: ${l2FeeRefundAddress}`);
    console.log(`ETH Amount (WEI): ${weiAmount}`);
    console.log(`Gas Limit: ${gasLimit}`);
    console.log(`Total Native Amount (WEI): ${total_native_amount}`);
    console.log(`User Max Fee Per Gas: ${userMaxFeePerGas}`);
    console.log(`User Max Priority Fee Per Gas: ${userMaxPriorityFeePerGas}`);

    console.log(`Bridging ${weiAmount} (WEI) to recipient: ${l2DepositRecipient}`);

    // get depositRecipientBalance
    const depositRecipientSmartAccount = await loadL2DepositRecipientSmartAccount();

    if (!depositRecipientSmartAccount) {
        throw Error(`Invalid depositRecipientSmartAccount`);
    }

    const balance = await depositRecipientSmartAccount.getBalance();

    let nilNetworkConfig: L2NetworkConfig = loadNilNetworkConfig("local");
    nilNetworkConfig.l2TestConfig.ethBalanceBefBridge = balance;
    saveNilNetworkConfig("local", nilNetworkConfig);

    // Perform the depositETH transaction
    const tx = await l1ETHBridgeInstance.depositETH(
        weiAmount,
        l2DepositRecipient,
        l2FeeRefundAddress,
        gasLimit,
        userMaxFeePerGas,
        userMaxPriorityFeePerGas,
        { value: totalNativeAmount }
    );

    await tx.wait();

    const transactionHash = tx.hash;

    console.log(`transactionHash for ETHDeposit is: ${transactionHash}`);

    const transactionDetails: TransactionReceipt = await source.provider.getTransactionReceipt(transactionHash);
    console.log(`transactionDetails are: ${JSON.stringify(transactionDetails)}`);
    if (!transactionDetails || transactionDetails.status == 0) {
        throw new Error(`DepositETH L1Bridge transaction failed`);
    } else {
        console.log(`Successful DepositETH transaction on L1ETHBridge`);
    }

    
    console.log(`DepositETH via L1ETHBridge costed -> cumlativeGasUsed : ${transactionDetails.cumulativeGasUsed} - gasUsed: ${transactionDetails.gasUsed}`);

    const messageSentEventLogData = await extractAndParseMessageSentEventLog(source.provider, transactionHash);

    if (!messageSentEventLogData) {
        throw new Error(`Failed to parse MessageSent event Log emitted by L1BridgeMessenger contract`);
    }

    const messageSentEvent: MessageSentEvent = messageSentEventLogData;

    console.log(`messageSentEvent for depositETH is: ${JSON.stringify(messageSentEvent, bigIntReplacer, 2)}`);

    // save the messageHash in the json config for l2
    const l2NetworkConfig: L2NetworkConfig = loadNilNetworkConfig("local");

    l2NetworkConfig.l2TestConfig.messageSentEvent = messageSentEvent;

    saveNilNetworkConfig("local", l2NetworkConfig);


    return messageSentEventLogData.messageHash;
}
