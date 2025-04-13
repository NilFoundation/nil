import { DeployFunction } from 'hardhat-deploy/types';
import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { deployWETHTokenContract } from './deploy-weth-token';

// npx hardhat deploy --network sepolia --tags WETHTokenDeploy
// npx hardhat deploy --network geth --tags WETHTokenDeploy
const deployWETHToken: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { deployments, getNamedAccounts, ethers, network } = hre;
    const { deploy } = deployments;
    const networkName = network.name;
    const { deployer } = await getNamedAccounts();
    await deployWETHTokenContract(networkName, deployer, deploy);
};

export default deployWETHToken;
deployWETHToken.tags = ['WETHTokenDeploy'];
