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
import { BatchInfo } from '../../config/nil-types';
import { getProxyAdminAddressWithRetry, verifyContractWithRetry } from '../../common/proxy-contract-utils';

// npx hardhat deploy --network sepolia --tags L1ERC20Bridge
// npx hardhat deploy --network anvil --tags L1ERC20Bridge
// npx hardhat deploy --network geth --tags L1ERC20Bridge
const deployL1ERC20Bridge: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { getNamedAccounts } = hre;
    const { deployer } = await getNamedAccounts();
    const networkName = network.name;
    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);

    // Validate configuration parameters
    if (!isValidAddress(config.l1ERC20Bridge.owner)) {
        throw new Error('Invalid owner in config');
    }
    if (!isValidAddress(config.l1ERC20Bridge.admin)) {
        throw new Error('Invalid admin in config');
    }

    // Check if L1ERC20Bridge is already deployed
    if (config.l1ERC20Bridge.l1ERC20BridgeProxy && isValidAddress(config.l1ERC20Bridge.l1ERC20BridgeProxy)) {
        console.log(`L1ERC20Bridge already deployed at: ${config.l1ERC20Bridge.l1ERC20BridgeProxy}`);
        archiveL1NetworkConfig(networkName, config);
    }

    try {
        // Deploy L1ERC20Bridge implementation
        const L1ERC20Bridge = await ethers.getContractFactory('L1ERC20Bridge');

        // proxy admin contract
        // deploys implementation contract (L1BridgeMessenger)
        // deploys ProxyContract
        // sets implementation contract address in the ProxyContract storage
        // initialize the contract
        // entire storage is owned by proxy contract
        const l1ERC20BridgeProxy = await upgrades.deployProxy(
            L1ERC20Bridge,
            [
                config.l1ERC20Bridge.owner, // _owner
                config.l1ERC20Bridge.admin, // _defaultAdmin
                config.l1Common.weth,
                config.l1BridgeMessengerConfig.l1BridgeMessengerProxy,
                config.nilGasPriceOracleConfig.nilGasPriceOracleProxy
            ],
            { initializer: 'initialize' },
        );

        console.log(`l1ERC20BridgeProxy deployed to: ${l1ERC20BridgeProxy.target}`);

        const l1ERC20BridgeProxyAddress = l1ERC20BridgeProxy.target;
        config.l1ERC20Bridge.l1ERC20BridgeProxy = l1ERC20BridgeProxyAddress;

        // query proxyAdmin address and implementation address
        const proxyAdminAddress = await getProxyAdminAddressWithRetry(
            l1ERC20BridgeProxyAddress,
        );
        config.l1ERC20Bridge.proxyAdmin = proxyAdminAddress;

        if (proxyAdminAddress === ZeroAddress) {
            throw new Error('Invalid proxy admin address');
        }

        const implementationAddress =
            await upgrades.erc1967.getImplementationAddress(
                l1ERC20BridgeProxyAddress,
            );
        config.l1ERC20Bridge.l1ERC20BridgeImplementation = implementationAddress;

        if (implementationAddress === ZeroAddress) {
            throw new Error('Invalid implementation address');
        }

        // Query the proxy storage and assert if the input arguments are correctly set in the contract storage
        const nilRollup = L1ERC20Bridge.attach(l1ERC20BridgeProxyAddress);

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
                await verifyContractWithRetry(l1ERC20BridgeProxyAddress, []);
            } catch (error) {
                console.error(
                    'L1ERC20Bridge Verification failed after retries:',
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

export default deployL1ERC20Bridge;
deployL1ERC20Bridge.tags = ['L1ERC20Bridge'];
