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
import { bigIntReplacer, extractAndParseMessageSentEventLog, MessageSentEvent } from './get-messenger-events';

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

// npx hardhat run scripts/bridge-test/cancel-deposit.ts --network geth
export async function cancelDeposit() {
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

    const messageHash = "0x2b53266fde3c8ce916ae55a8d737f3211c7a0de44078dbe08f0419d76fb945c1";
    const tx = await l1ERC20BridgeInstance.cancelDeposit(messageHash);
    const transactionReceipt: TransactionReceipt = await tx.wait();

    if (!transactionReceipt || transactionReceipt.status == 0) {
        throw new Error(`ERC20 Bridge transaction failed`);
    } else {
        console.log(`Successful ERC20DepositCancel transaction on L1ERC20Bridge`);
    }

    console.log(`CancelDeposit via L1ERC20Bridge costed -> cumlativeGasUsed : ${transactionReceipt.cumulativeGasUsed} - gasUsed: ${transactionReceipt.gasUsed}`);
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
    await cancelDeposit();
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
