import { DeployFunction } from 'hardhat-deploy/types';
import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { deployWETHTokenContract } from './token/deploy-weth-token';
import { deployERC20TokenContracts } from './token/deploy-erc20-tokens';
import { deployL2MockERC20TokenContracts } from './token/deploy-l2-mock-erc20-tokens';
import { deployMockL2BridgeContract } from './bridge/l1/mocks/deploy-mock-bridge-contract';

// npx hardhat deploy --network geth --tags DeployL1Mock
const deployMaster: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { deployments, getNamedAccounts, network } = hre;
    const { deploy } = deployments;
    const networkName = network.name;
    const { deployer } = await getNamedAccounts();
    await deployWETHTokenContract(networkName, deployer, deploy);
    await deployERC20TokenContracts(networkName, deployer, deploy);
    await deployL2MockERC20TokenContracts(networkName, deployer, deploy);
    await deployMockL2BridgeContract(networkName, deployer, deploy);
};

export default deployMaster;
deployMaster.tags = ['DeployL1Mock'];
