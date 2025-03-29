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
} from '../../config/config-helper';
import { getProxyAdminAddressWithRetry, verifyContractWithRetry } from '../../common/proxy-contract-utils';

// npx hardhat deploy --network sepolia --tags L1BridgeRouter
// npx hardhat deploy --network anvil --tags L1BridgeRouter
// npx hardhat deploy --network geth --tags L1BridgeRouter
const deployL1BridgeRouter: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { getNamedAccounts } = hre;
    const { deployer } = await getNamedAccounts();
    const networkName = network.name;
    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);

    // Validate configuration parameters
    if (!isValidAddress(config.l1Common.owner)) {
        throw new Error('Invalid owner in config');
    }
    if (!isValidAddress(config.l1Common.admin)) {
        throw new Error('Invalid admin in config');
    }
    if (!isValidAddress(config.l1ERC20Bridge.l1ERC20BridgeProxy)) {
        throw new Error('Invalid L1ERC20BridgeProxy in config');
    }
    if (!isValidAddress(config.l1ETHBridgeConfig.l1ETHBridgeProxy)) {
        throw new Error('Invalid L1ETHBridgeProxy in config');
    }
    if (!isValidAddress(config.l1BridgeMessengerConfig.l1BridgeMessengerProxy)) {
        throw new Error('Invalid L1BridgeMessengerProxy in config');
    }
    if (!isValidAddress(config.l1Common.weth)) {
        throw new Error('Invalid WETH in config');
    }

    // Check if L1BridgeRouter is already deployed
    if (config.l1BridgeRouterConfig.l1BridgeRouterProxy && isValidAddress(config.l1BridgeRouterConfig.l1BridgeRouterProxy)) {
        console.log(`L1BridgeRouter already deployed at: ${config.l1BridgeRouterConfig.l1BridgeRouterProxy}`);
        archiveL1NetworkConfig(networkName, config);
    }

    try {
        // Deploy L1BridgeRouter implementation
        const L1BridgeRouter = await ethers.getContractFactory('L1BridgeRouter');

        // proxy admin contract
        // deploys implementation contract (L1BridgeRouter)
        // deploys ProxyContract
        // sets implementation contract address in the ProxyContract storage
        // initialize the contract
        // entire storage is owned by proxy contract
        const l1BridgeRouterProxy = await upgrades.deployProxy(
            L1BridgeRouter,
            [
                config.l1Common.owner, // _owner
                config.l1Common.admin, // _defaultAdmin
                config.l1ERC20Bridge.l1ERC20BridgeProxy,
                config.l1ETHBridgeConfig.l1ETHBridgeProxy,
                config.l1BridgeMessengerConfig.l1BridgeMessengerProxy,
                config.l1Common.weth
            ],
            { initializer: 'initialize' },
        );

        console.log(`l1BridgeRouterProxy-Proxy deployed to: ${l1BridgeRouterProxy.target}`);

        const l1BridgeRouterProxyAddress = l1BridgeRouterProxy.target;
        config.l1BridgeRouterConfig.l1BridgeRouterProxy = l1BridgeRouterProxyAddress;

        // query proxyAdmin address and implementation address
        const proxyAdminAddress = await getProxyAdminAddressWithRetry(
            l1BridgeRouterProxyAddress,
        );
        config.l1BridgeRouterConfig.proxyAdmin = proxyAdminAddress;

        if (proxyAdminAddress === ZeroAddress) {
            throw new Error('Invalid proxy admin address');
        }

        const implementationAddress =
            await upgrades.erc1967.getImplementationAddress(
                l1BridgeRouterProxyAddress,
            );
        config.l1BridgeRouterConfig.l1BridgeRouterImplementation = implementationAddress;

        if (implementationAddress === ZeroAddress) {
            throw new Error('Invalid implementation address');
        }

        // Query the proxy storage and assert if the input arguments are correctly set in the contract storage
        const nilRollup = L1BridgeRouter.attach(l1BridgeRouterProxyAddress);

        // Save the updated config
        saveL1NetworkConfig(networkName, config);

        // check network and verify if its not geth or anvil
        // Skip verification if the network is local or anvil
        if (
            network.name !== 'local' &&
            network.name !== 'anvil' &&
            network.name !== 'geth'
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
    }
};

export default deployL1BridgeRouter;
deployL1BridgeRouter.tags = ['L1BridgeRouter'];
