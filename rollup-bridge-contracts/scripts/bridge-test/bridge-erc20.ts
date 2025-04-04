import { ethers, network } from 'hardhat';
import { Contract, TransactionReceipt } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadL1NetworkConfig,
    isValidAddress,
    ERC20TokenContract,
    loadL1MockConfig,
} from '../../deploy/config/config-helper';
import { bigIntReplacer, DepositERC20Event, extractAndParseDepositERC20Event, extractAndParseMessageSentEventLog, MessageSentEvent } from './get-messenger-events';

const l1ERC20BridgeABIPath = path.join(
    __dirname,
    '../../artifacts/contracts/bridge/l1/interfaces/IL1ERC20Bridge.sol/IL1ERC20Bridge.json',
);
const l1ERC20BridgeABI = JSON.parse(fs.readFileSync(l1ERC20BridgeABIPath, 'utf8')).abi;

const erc20ABIPath = path.join(
    __dirname,
    '../../artifacts/contracts/common/tokens/TestERC20.sol/TestERC20Token.json',
);
const erc20ABI = JSON.parse(fs.readFileSync(erc20ABIPath, 'utf8')).abi;

// npx hardhat run scripts/bridge-test/bridge-erc20.ts --network geth
export async function bridgeERC20() {
    const networkName = network.name;
    const config = loadL1NetworkConfig(networkName);

    if (!isValidAddress(config.l1ERC20Bridge.l1ERC20BridgeProxy)) {
        throw new Error('Invalid l1ERC20BridgeProxy address in config');
    }
    const signers = await ethers.getSigners();

    const signer = signers[0]; // The main signer

    const signerAddress = signer.address;
    const l1ERC20BridgeInstance = new ethers.Contract(
        config.l1ERC20Bridge.l1ERC20BridgeProxy,
        l1ERC20BridgeABI,
        signer,
    ) as Contract;

    const recipientAddress = "0x66bFaD51E02513C5B6bEfe1Acc9a31Cb6eE152F1";
    const token_amount = 10000;
    const l2FeeRefundRecipientAddress = "0x878f824Ffde85B7Bd6ad6c6Fd97275bb6724c55a";
    const gasLimit = 1000;
    const total_native_amount = gasLimit * 1000;
    const userFeePerGas = 0;
    const userMaxPriorityFeePerGas = 0;
    const symbol = "USDC";

    const l1MockConfig = loadL1MockConfig(networkName);
    const erc20TokenData = getERC20TokenBySymbol(l1MockConfig.tokens, symbol);
    //const mockL2TokenData = get

    if (erc20TokenData == null || !erc20TokenData) {
        throw new Error(`Invalid TokenData`);
    }

    const erc20TokenInstance = new ethers.Contract(
        erc20TokenData.address,
        erc20ABI,
        signer,
    ) as Contract;

    // signer to mint ERC20 tokens and approve the spending to config.l1ERC20BridgeConfig.l1ERC20BridgeProxy

    const mintTx = await erc20TokenInstance.mint(signerAddress, token_amount);
    await mintTx.wait();

    const tokenBalance = await erc20TokenInstance.balanceOf(signerAddress);
    const approveTxn = await erc20TokenInstance.approve(config.l1ERC20Bridge.l1ERC20BridgeProxy, tokenBalance);
    await approveTxn.wait();

    await erc20TokenInstance.allowance(signer.address, config.l1ERC20Bridge.l1ERC20BridgeProxy);

    console.log(`bridging ${token_amount} (WEI) - ${erc20TokenData.erc20TokenInitConfig.symbol} to recipient: ${recipientAddress} and with l2FeeRefundRecipientAddress: ${l2FeeRefundRecipientAddress}`);

    const tx = await l1ERC20BridgeInstance.depositERC20(erc20TokenData.address, recipientAddress, token_amount, l2FeeRefundRecipientAddress, gasLimit, userFeePerGas, userMaxPriorityFeePerGas, { value: total_native_amount });
    const transactionReceipt: TransactionReceipt = await tx.wait();

    if (!transactionReceipt || transactionReceipt.status == 0) {
        throw new Error(`ERC20 Bridge transaction failed`);
    } else {
        console.log(`Successful ERC20Deposit transaction on L1ERC20Bridge`);
    }

    const transactionHash = tx.hash;
    const messageSentEventLogData = await extractAndParseMessageSentEventLog(transactionHash);

    if (!messageSentEventLogData) {
        throw new Error(`Failed to parse MessageSent event Log emitted by L1BridgeMessenger contract`);
    }

    const messageSentEvent: MessageSentEvent = messageSentEventLogData;

    console.log(`ERC20Bridging transaction has emitted MessageSentEvent: ${JSON.stringify(messageSentEvent, bigIntReplacer, 2)}`);

    const depositERC20EventLogData = await extractAndParseDepositERC20Event(transactionHash);

    if (!depositERC20EventLogData) {
        throw new Error(`Failed to parse DepositERC20Event Log emitted by L1ERC20Bridge contract`);
    }

    const depositERC20Event: DepositERC20Event = depositERC20EventLogData;

    console.log(`ERC20Bridging transaction has emitted DepositERC20Event: ${JSON.stringify(depositERC20Event, bigIntReplacer, 2)}`);
}

function getERC20TokenBySymbol(tokens: ERC20TokenContract[], symbol: string): ERC20TokenContract | null {
    for (const token of tokens) {
        if (token.erc20TokenInitConfig.symbol === symbol) {
            return token;
        }
    }

    return null;
}

async function main() {
    await bridgeERC20();
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
