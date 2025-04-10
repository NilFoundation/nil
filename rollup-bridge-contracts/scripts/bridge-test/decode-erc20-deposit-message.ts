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

// npx hardhat run scripts/bridge-test/decode-erc20-deposit-message.ts --network geth
export async function decodeERC20DepositMessage() {
    const networkName = network.name;
    const config = loadL1NetworkConfig(networkName);

    if (!isValidAddress(config.l1ERC20Bridge.l1ERC20BridgeProxy)) {
        throw new Error('Invalid l1ERC20BridgeProxy address in config');
    }
    const signers = await ethers.getSigners();
    const signer = signers[0]; // The main signer
    const l1ERC20BridgeInstance = new ethers.Contract(
        config.l1ERC20Bridge.l1ERC20BridgeProxy,
        l1ERC20BridgeABI,
        signer,
    ) as Contract;

    const messageHash = "0x81508701daba548fba097fe1b8b76da57658a3d3c61b33c34a8022c3873c356e";
    const decodedMessage = await l1ERC20BridgeInstance.decodeBridgeData(messageHash);

    console.log(`DepositERC20 Message is decoded as : ${JSON.stringify(decodedMessage)}`);
}

async function main() {
    await decodeERC20DepositMessage();
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
