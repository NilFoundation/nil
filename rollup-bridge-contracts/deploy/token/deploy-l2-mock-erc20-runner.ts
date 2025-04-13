import { DeployFunction } from 'hardhat-deploy/types';
import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { deployL2MockERC20TokenContracts } from './deploy-l2-mock-erc20-tokens';

// npx hardhat deploy --network geth --tags MockL2ERC20TokensDeploy
const deployMockL2ERC20Tokens: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { deployments, getNamedAccounts, ethers, network } = hre;
    const { deploy } = deployments;
    const networkName = network.name;
    const { deployer } = await getNamedAccounts();
    await deployL2MockERC20TokenContracts(networkName, deployer, deploy);
};

export default deployMockL2ERC20Tokens;
deployMockL2ERC20Tokens.tags = ['MockL2ERC20TokensDeploy'];
