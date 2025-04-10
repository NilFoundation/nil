import { DeployFunction } from 'hardhat-deploy/types';
import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { deployRollupContracts } from './rollup/deploy-rollup-contracts';
import { deployNilGasPriceOracleContract } from './bridge/l1/oracle/deploy-nil-gas-price-oracle-contract';
import { deployL1BridgeMessengerContract } from './bridge/l1/messenger/deploy-bridge-messenger-contract';
import { deployL1ETHBridgeContract } from './bridge/l1/eth/deploy-eth-bridge-contract';
import { deployL1ERC20BridgeContract } from './bridge/l1/erc20/deploy-erc20-bridge-contract';
import { deployL1BridgeRouterContract } from './bridge/l1/router/deploy-bridge-router-contract';

// npx hardhat deploy --network geth --tags DeployL1Master
const deployMaster: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { deployments, getNamedAccounts, network } = hre;
    const { deploy } = deployments;
    const networkName = network.name;
    const { deployer } = await getNamedAccounts();
    await deployRollupContracts(networkName, deployer, deploy);
    await deployNilGasPriceOracleContract(networkName);
    await deployL1BridgeMessengerContract(networkName);
    await deployL1ETHBridgeContract(networkName);
    await deployL1ERC20BridgeContract(networkName);
    await deployL1BridgeRouterContract(networkName);
};

export default deployMaster;
deployMaster.tags = ['DeployL1Master'];
