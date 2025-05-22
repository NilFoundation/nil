import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { DeployFunction } from 'hardhat-deploy/types';
import { deployNilGasPriceOracleContract } from './deploy-nil-gas-price-oracle-contract';

// npx hardhat deploy --network sepolia --tags NilGasPriceOracle
// npx hardhat deploy --network geth --tags NilGasPriceOracle
const deployNilGasPriceOracle: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    // Lazy import inside the function
    // @ts-ignore
    const { ethers, network, upgrades, run } = await import('hardhat');

    // @ts-ignore
    const { getNamedAccounts } = hre;
    const { deployer } = await getNamedAccounts();
    const networkName = network.name;
    await deployNilGasPriceOracleContract(networkName);
};

export default deployNilGasPriceOracle;
deployNilGasPriceOracle.tags = ['NilGasPriceOracle'];
