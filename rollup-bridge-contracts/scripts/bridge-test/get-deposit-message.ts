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

const l1BridgeMessengerABIPath = path.join(
    __dirname,
    '../../artifacts/contracts/bridge/l1/interfaces/IL1BridgeMessenger.sol/IL1BridgeMessenger.json',
);
const l1BridgeMessengerABI = JSON.parse(fs.readFileSync(l1BridgeMessengerABIPath, 'utf8')).abi;

// npx hardhat run scripts/bridge-test/get-deposit-message.ts --network geth
export async function getDepositMessage(messageHash: string) {
    const networkName = network.name;
    const config = loadL1NetworkConfig(networkName);

    if (!isValidAddress(config.l1ERC20Bridge.l1ERC20BridgeProxy)) {
        throw new Error('Invalid l1ERC20BridgeProxy address in config');
    }
    const signers = await ethers.getSigners();

    const signer = signers[0]; // The main signer

    const signerAddress = signer.address;
    const l1BridgeMessengerInstance = new ethers.Contract(
        config.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerProxy,
        l1BridgeMessengerABI,
        signer,
    ) as Contract;

    const messageDetails = await l1BridgeMessengerInstance.getDepositMessage(messageHash);

    console.log(`messageDetails are: ${JSON.stringify(messageDetails, bigIntReplacer, 2)}`);
}

async function main() {
    const messageHash = "0x618368c9710dd3f7c5a92225c77b2c9f27288a4e2f36ff839b9823a547429aa2";
    await getDepositMessage(messageHash);
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
