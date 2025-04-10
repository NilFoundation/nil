import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { DeployFunction } from 'hardhat-deploy/types';
import { ethers, network, upgrades, run } from 'hardhat';
import { deployL1BridgeRouterContract } from './deploy-bridge-router-contract';

// npx hardhat deploy --network sepolia --tags L1BridgeRouter
// npx hardhat deploy --network geth --tags L1BridgeRouter
const deployL1BridgeRouter: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { getNamedAccounts } = hre;
    const { deployer } = await getNamedAccounts();
    const networkName = network.name;
    await deployL1BridgeRouterContract(networkName);
};

export default deployL1BridgeRouter;
deployL1BridgeRouter.tags = ['L1BridgeRouter'];
