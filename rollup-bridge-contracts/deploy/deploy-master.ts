import { DeployFunction } from 'hardhat-deploy/types';
import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { ethers, upgrades, run } from 'hardhat';
import {
    archiveL1NetworkConfig,
    isValidAddress,
    isValidBytes32,
    L1NetworkConfig,
    loadL1NetworkConfig,
    saveL1NetworkConfig,
    ZeroAddress,
} from './config/config-helper';
import { BatchInfo, proposerRoleHash } from './config/nil-types';
import { getProxyAdminAddressWithRetry, verifyContractWithRetry } from './common/proxy-contract-utils';
import { deployRollupContracts } from './rollup/deploy-rollup-contracts';
import { deployNilGasPriceOracleContract } from './bridge/l1/oracle/deploy-nil-gas-price-oracle-contract';
import { deployL1BridgeMessengerContract } from './bridge/l1/messenger/deploy-bridge-messenger-contract';
import { deployWETHTokenContract } from './token/deploy-weth-token';
import deployERC20Tokens, { deployERC20TokenContracts } from './token/deploy-erc20-tokens';
import { deployL2MockERC20TokenContracts } from './token/deploy-l2-mock-erc20-tokens';
import { deployMockL2BridgeContract } from './bridge/l1/mocks/deploy-mock-bridge-contract';
import { deployL1ETHBridgeContract } from './bridge/l1/eth/deploy-eth-bridge-contract';
import { deployL1ERC20BridgeContract } from './bridge/l1/erc20/deploy-erc20-bridge-contract';
import { deployL1BridgeRouterContract } from './bridge/l1/router/deploy-bridge-router-contract';

// npx hardhat deploy --network geth --tags DeployMaster
// npx hardhat deploy --network sepolia --tags DeployMaster
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
    await deployRollupContracts(networkName, deployer, deploy);
    await deployNilGasPriceOracleContract(networkName);
    await deployL1BridgeMessengerContract(networkName);
    await deployMockL2BridgeContract(networkName, deployer, deploy);
    await deployL1ETHBridgeContract(networkName);
    await deployL1ERC20BridgeContract(networkName);
    await deployL1BridgeRouterContract(networkName);
};

export default deployMaster;
deployMaster.tags = ['DeployMaster'];
