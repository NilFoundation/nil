// @ts-ignore
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

    if (!isValidAddress(config.nilGasPriceOracle.nilGasPriceOracleContracts.nilGasPriceOracleProxy)) {
        throw new Error('Invalid nilGasPriceOracleProxy address in config');
    }

    const [signer] = await ethers.getSigners();

    const nilGasPriceOracleInstance = new ethers.Contract(
        config.nilGasPriceOracle.nilGasPriceOracleContracts.nilGasPriceOracleProxy,
        nilGasPriceOracleABI,
        signer,
    ) as Contract;

    console.log(`setting user-gas-gee in nilGasPriceOracle`);

    const tx = await nilGasPriceOracleInstance.setOracleFee(config.nilGasPriceOracle.nilGasPriceOracleDeployerConfig.nilGasPriceOracleMaxFeePerGas,
        config.nilGasPriceOracle.nilGasPriceOracleDeployerConfig.nilGasPriceOracleMaxPriorityFeePerGas);
    await tx.wait();

    console.log(`nilGasPriceOracleMaxPriorityFeePerGas set in nilGasPriceOracle with transaction: ${JSON.stringify(tx)}`);

    console.log(`completed setting user-gas-fees in nilGasPriceOracle`);
}
