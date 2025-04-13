import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { DeployFunction } from 'hardhat-deploy/types';
import { ethers, network, upgrades, run } from 'hardhat';
import {
    archiveL1NetworkConfig,
    isValidAddress,
    isValidBytes32,
    L1NetworkConfig,
    loadL1NetworkConfig,
    saveL1NetworkConfig,
    ZeroAddress,
} from '../../../config/config-helper';
import { getProxyAdminAddressWithRetry, verifyContractWithRetry } from '../../../common/proxy-contract-utils';

export async function deployL1BridgeRouterContract(networkName: string): Promise<void> {
    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);
    // Validate configuration parameters
    if (!isValidAddress(config.l1DeployerConfig.owner)) {
        throw new Error('Invalid owner in config');
    }
    if (!isValidAddress(config.l1DeployerConfig.admin)) {
        throw new Error('Invalid admin in config');
    }
    if (!isValidAddress(config.l1ERC20Bridge.l1ERC20BridgeProxy)) {
        throw new Error('Invalid L1ERC20BridgeProxy in config');
    }
    if (!isValidAddress(config.l1ETHBridge.l1ETHBridgeProxy)) {
        throw new Error('Invalid L1ETHBridgeProxy in config');
    }
    if (!isValidAddress(config.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerProxy)) {
        throw new Error('Invalid L1BridgeMessengerProxy in config');
    }
    if (!isValidAddress(config.l1CommonContracts.weth)) {
        throw new Error('Invalid WETH in config');
    }

    try {
        // Deploy L1BridgeRouter implementation
        const L1BridgeRouter = await ethers.getContractFactory('L1BridgeRouter');

        // Deploy proxy admin contract and initialize the proxy
        const l1BridgeRouterProxy = await upgrades.deployProxy(
            L1BridgeRouter,
            [
                config.l1DeployerConfig.owner, // _owner
                config.l1DeployerConfig.admin, // _defaultAdmin
                config.l1ERC20Bridge.l1ERC20BridgeProxy,
                config.l1ETHBridge.l1ETHBridgeProxy,
                config.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerProxy,
                config.l1CommonContracts.weth
            ],
            { initializer: 'initialize' },
        );

        console.log(`l1BridgeRouterProxy-Proxy deployed to: ${l1BridgeRouterProxy.target}`);

        const l1BridgeRouterProxyAddress = l1BridgeRouterProxy.target;
        config.l1BridgeRouter.l1BridgeRouterProxy = l1BridgeRouterProxyAddress;

        // Query proxyAdmin address and implementation address
        const proxyAdminAddress = await getProxyAdminAddressWithRetry(
            l1BridgeRouterProxyAddress,
        );
        config.l1BridgeRouter.proxyAdmin = proxyAdminAddress;

        if (proxyAdminAddress === ZeroAddress) {
            throw new Error('Invalid proxy admin address');
        }

        const implementationAddress =
            await upgrades.erc1967.getImplementationAddress(
                l1BridgeRouterProxyAddress,
            );
        config.l1BridgeRouter.l1BridgeRouterImplementation = implementationAddress;

        if (implementationAddress === ZeroAddress) {
            throw new Error('Invalid implementation address');
        }

        // Query the proxy storage and assert if the input arguments are correctly set in the contract storage
        const l1BridgeRouterContractInstance = L1BridgeRouter.attach(l1BridgeRouterProxyAddress);

        // Save the updated config
        saveL1NetworkConfig(networkName, config);

        // Check network and verify if it's not geth or anvil
        if (
            networkName !== 'local' &&
            networkName !== 'anvil' &&
            networkName !== 'geth'
        ) {
            try {
                await verifyContractWithRetry(l1BridgeRouterProxyAddress, []);
            } catch (error) {
                console.error(
                    'L1BridgeRouter Verification failed after retries:',
                    error,
                );
            }
        } else {
            console.log('Skipping verification on local or anvil network');
        }
    } catch (error) {
        console.error('Error during deployment:', error);
        throw new Error(`Error while deploying L1BridgeRouter on network: ${networkName} - ${error}`);
    }
}
