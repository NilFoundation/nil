import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { DeployFunction } from 'hardhat-deploy/types';
import { ethers, network, upgrades, run } from 'hardhat';
import { deployL1ERC20BridgeContract } from './deploy-erc20-bridge-contract';

// npx hardhat deploy --network sepolia --tags L1ERC20Bridge
// npx hardhat deploy --network geth --tags L1ERC20Bridge
const deployL1ERC20Bridge: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { getNamedAccounts } = hre;
    const { deployer } = await getNamedAccounts();
    const networkName = network.name;
    await deployL1ERC20BridgeContract(networkName);
};

export default deployL1ERC20Bridge;
deployL1ERC20Bridge.tags = ['L1ERC20Bridge'];
