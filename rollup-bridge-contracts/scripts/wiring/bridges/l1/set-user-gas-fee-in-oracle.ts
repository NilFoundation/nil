import { ethers, network } from 'hardhat';
import { Contract } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadL1NetworkConfig,
    isValidAddress,
} from '../../../../deploy/config/config-helper';

const nilGasPriceOracleABIPath = path.join(
    __dirname,
    '../../../../artifacts/contracts/bridge/l1/interfaces/INilGasPriceOracle.sol/INilGasPriceOracle.json',
);
const nilGasPriceOracleABI = JSON.parse(fs.readFileSync(nilGasPriceOracleABIPath, 'utf8')).abi;

// npx hardhat run scripts/wiring/bridges/l1/set-user-gas-fee-in-oracle.ts --network geth
export async function setUserGasFeeInOracle() {
    const networkName = network.name;
    const config = loadL1NetworkConfig(networkName);

    // setMaxFeePerGas
    // setMaxPriorityFeePerGas

    if (!isValidAddress(config.nilGasPriceOracleConfig.nilGasPriceOracleProxy)) {
        throw new Error('Invalid nilGasPriceOracleProxy address in config');
    }

    const [signer] = await ethers.getSigners();

    const nilGasPriceOracleInstance = new ethers.Contract(
        config.nilGasPriceOracleConfig.nilGasPriceOracleProxy,
        nilGasPriceOracleABI,
        signer,
    ) as Contract;

    const tx = await nilGasPriceOracleInstance.setMaxFeePerGas(config.nilGasPriceOracleConfig.nilGasPriceOracleMaxFeePerGas);
    await tx.wait();

    const tx2 = await nilGasPriceOracleInstance.setMaxPriorityFeePerGas(config.nilGasPriceOracleConfig.nilGasPriceOracleMaxPriorityFeePerGas);
    await tx2.wait();
}

async function main() {
    await setUserGasFeeInOracle();
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
