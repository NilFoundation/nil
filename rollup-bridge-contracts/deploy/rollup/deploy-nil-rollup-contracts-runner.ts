import { DeployFunction } from 'hardhat-deploy/types';
import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { deployRollupContracts } from './deploy-rollup-contracts';

// npx hardhat deploy --network geth --tags NilRollupContracts
// npx hardhat deploy --network sepolia --tags NilRollupContracts
const deployNilRollupContracts: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { deployments, getNamedAccounts, network } = hre;
    const { deploy } = deployments;
    const networkName = network.name;
    const { deployer } = await getNamedAccounts();
    await deployRollupContracts(networkName, deployer, deploy);
};

export default deployNilRollupContracts;
deployNilRollupContracts.tags = ['NilRollupContracts'];
