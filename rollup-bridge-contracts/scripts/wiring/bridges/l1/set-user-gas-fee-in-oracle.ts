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

export async function setUserGasFeeInOracle(networkName: string) {
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

    console.log(`setting user-gas-gee in nilGasPriceOracle`);

    const tx = await nilGasPriceOracleInstance.setMaxFeePerGas(config.nilGasPriceOracleConfig.nilGasPriceOracleMaxFeePerGas);
    await tx.wait();

    console.log(`nilGasPriceOracleMaxFeePerGas set in nilGasPriceOracle with transaction: ${JSON.stringify(tx)}`);

    const tx2 = await nilGasPriceOracleInstance.setMaxPriorityFeePerGas(config.nilGasPriceOracleConfig.nilGasPriceOracleMaxPriorityFeePerGas);
    await tx2.wait();

    console.log(`nilGasPriceOracleMaxPriorityFeePerGas set in nilGasPriceOracle with transaction: ${JSON.stringify(tx2)}`);

    console.log(`completed setting user-gas-fees in nilGasPriceOracle`);
}
