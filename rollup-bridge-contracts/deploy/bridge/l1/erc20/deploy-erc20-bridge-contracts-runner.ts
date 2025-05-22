import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { DeployFunction } from 'hardhat-deploy/types';
import { deployL1ERC20BridgeContract } from './deploy-erc20-bridge-contract';

// npx hardhat deploy --network sepolia --tags L1ERC20Bridge
// npx hardhat deploy --network geth --tags L1ERC20Bridge
const deployL1ERC20Bridge: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    // Lazy import inside the function
    // @ts-ignore
    const { ethers, network, upgrades, run } = await import('hardhat');

    // @ts-ignore
    const { getNamedAccounts } = hre;
    const { deployer } = await getNamedAccounts();
    const networkName = network.name;
    await deployL1ERC20BridgeContract(networkName);
};

export default deployL1ERC20Bridge;
deployL1ERC20Bridge.tags = ['L1ERC20Bridge'];
