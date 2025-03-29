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

// npx hardhat deploy --network sepolia --tags L1ETHBridge
// npx hardhat deploy --network anvil --tags L1ETHBridge
// npx hardhat deploy --network geth --tags L1ETHBridge
const deployL1ETHBridge: DeployFunction = async function (
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

    // Check if L1ETHBridge is already deployed
    if (config.l1ETHBridgeConfig.l1ETHBridgeProxy && isValidAddress(config.l1ETHBridgeConfig.l1ETHBridgeProxy)) {
        console.log(`L1ETHBridge already deployed at: ${config.l1ETHBridgeConfig.l1ETHBridgeProxy}`);
        archiveL1NetworkConfig(networkName, config);
    }

    try {
        // Deploy L1ETHBridge implementation
        const L1ETHBridge = await ethers.getContractFactory('L1ETHBridge');

        // proxy admin contract
        // deploys implementation contract (L1ETHBridge)
        // deploys ProxyContract
        // sets implementation contract address in the ProxyContract storage
        // initialize the contract
        // entire storage is owned by proxy contract
        const l1ETHBridgeProxy = await upgrades.deployProxy(
            L1ETHBridge,
            [
                config.l1Common.owner, // _owner
                config.l1Common.admin, // _defaultAdmin
                config.l1BridgeMessengerConfig.l1BridgeMessengerProxy,
                config.nilGasPriceOracleConfig.nilGasPriceOracleProxy
            ],
            { initializer: 'initialize' },
        );

        console.log(`l1ETHBridgeProxy deployed to: ${l1ETHBridgeProxy.target}`);

        const l1ETHBridgeProxyAddress = l1ETHBridgeProxy.target;
        config.l1ETHBridgeConfig.l1ETHBridgeProxy = l1ETHBridgeProxyAddress;

        // query proxyAdmin address and implementation address
        const proxyAdminAddress = await getProxyAdminAddressWithRetry(
            l1ETHBridgeProxyAddress,
        );
        config.l1ETHBridgeConfig.proxyAdmin = proxyAdminAddress;

        if (proxyAdminAddress === ZeroAddress) {
            throw new Error('Invalid proxy admin address');
        }

        const implementationAddress =
            await upgrades.erc1967.getImplementationAddress(
                l1ETHBridgeProxyAddress,
            );
        config.l1ETHBridgeConfig.l1ETHBridgeImplementation = implementationAddress;

        if (implementationAddress === ZeroAddress) {
            throw new Error('Invalid implementation address');
        }

        // Query the proxy storage and assert if the input arguments are correctly set in the contract storage
        const nilRollup = L1ETHBridge.attach(l1ETHBridgeProxyAddress);

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
                await verifyContractWithRetry(l1ETHBridgeProxyAddress, []);
            } catch (error) {
                console.error(
                    'L1ETHBridge Verification failed after retries:',
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

export default deployL1ETHBridge;
deployL1ETHBridge.tags = ['L1ETHBridge'];
