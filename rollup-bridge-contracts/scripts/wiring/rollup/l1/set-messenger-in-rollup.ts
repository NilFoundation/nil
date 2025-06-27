import { Contract } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadL1NetworkConfig,
    isValidAddress,
} from '../../../../deploy/config/config-helper';

const rollupABIPath = path.join(
  __dirname,
  '../../../../artifacts/contracts/NilRollup.sol/NilRollup.json',
);
const rollupABI = JSON.parse(fs.readFileSync(rollupABIPath, 'utf8')).abi;

// npx hardhat run scripts/wiring/rollup/l1/set-messenger-in-rollup.ts --network geth
export async function setMessengerInRollup(networkName: string) {
    // Lazy import inside the function
    // @ts-ignore
    const { ethers } = await import('hardhat');
    const config = loadL1NetworkConfig(networkName);

    if (!isValidAddress(config.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerProxy)) {
        throw new Error('Invalid l1BridgeMessengerProxy address in config');
    }

    if (!isValidAddress(config.nilRollup.nilRollupContracts.nilRollupProxy)) {
      throw new Error('Invalid nilRollupProxy address in config');
    }

    const messengerAddress = config.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerProxy;
    const [signer] = await ethers.getSigners();

    const rollupInstance = new ethers.Contract(
        config.nilRollup.nilRollupContracts.nilRollupProxy,
        rollupABI,
        signer,
    ) as Contract;

    const rollupTx = await rollupInstance.setL1BridgeMessenger(messengerAddress);
    await rollupTx.wait();
    console.log(`messenger set in nil_rollup is: ${messengerAddress}`);
}
