import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { DeployFunction } from 'hardhat-deploy/types';
import { ethers, network, upgrades, run } from 'hardhat';
import { deployNilGasPriceOracleContract } from './deploy-nil-gas-price-oracle-contract';

// npx hardhat deploy --network sepolia --tags NilGasPriceOracle
// npx hardhat deploy --network geth --tags NilGasPriceOracle
const deployNilGasPriceOracle: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { getNamedAccounts } = hre;
    const { deployer } = await getNamedAccounts();
    const networkName = network.name;
    await deployNilGasPriceOracleContract(networkName);
};

export default deployNilGasPriceOracle;
deployNilGasPriceOracle.tags = ['NilGasPriceOracle'];
