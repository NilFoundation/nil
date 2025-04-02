import { DeployFunction } from 'hardhat-deploy/types';
import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { deployERC20TokenContracts } from './deploy-erc20-tokens';

// npx hardhat deploy --network sepolia --tags ERC20TokensDeploy
// npx hardhat deploy --network geth --tags ERC20TokensDeploy
const deployERC20Tokens: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { deployments, getNamedAccounts, ethers, network } = hre;
    const { deploy } = deployments;
    const networkName = network.name;
    const { deployer } = await getNamedAccounts();
    await deployERC20TokenContracts(networkName, deployer, deploy);
};

export default deployERC20Tokens;
deployERC20Tokens.tags = ['ERC20TokensDeploy'];
