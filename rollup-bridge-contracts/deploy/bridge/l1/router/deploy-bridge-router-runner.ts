import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { DeployFunction } from 'hardhat-deploy/types';
import { deployL1BridgeRouterContract } from './deploy-bridge-router-contract';

// npx hardhat deploy --network sepolia --tags L1BridgeRouter
// npx hardhat deploy --network geth --tags L1BridgeRouter
const deployL1BridgeRouter: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    // Lazy import inside the function
    // @ts-ignore
    const { ethers, network, upgrades, run } = await import('hardhat');

    // @ts-ignore
    const { getNamedAccounts } = hre;
    const { deployer } = await getNamedAccounts();
    const networkName = network.name;
    await deployL1BridgeRouterContract(networkName);
};

export default deployL1BridgeRouter;
deployL1BridgeRouter.tags = ['L1BridgeRouter'];
