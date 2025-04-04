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

// npx hardhat run scripts/bridge-test/cancel-deposit.ts --network geth
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

    const messageHash = "0x11642b3858c80ef20f385798e73ca27c58a2b42f3d2530c8af5672a2bf07653d";
    const tx = await l1ERC20BridgeInstance.cancelDeposit(messageHash);
    const transactionReceipt: TransactionReceipt = await tx.wait();

    if (!transactionReceipt || transactionReceipt.status == 0) {
        throw new Error(`ERC20 Bridge transaction failed`);
    } else {
        console.log(`Successful ERC20Deposit transaction on L1ERC20Bridge`);
    }

    console.log(`DepositERC20 via L1ERC20Bridge costed -> cumlativeGasUsed : ${transactionReceipt.cumulativeGasUsed} - gasUsed: ${transactionReceipt.gasUsed}`);
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
