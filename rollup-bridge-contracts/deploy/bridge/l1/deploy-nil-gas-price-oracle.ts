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

// npx hardhat deploy --network sepolia --tags L1BridgeMessenger
// npx hardhat deploy --network anvil --tags L1BridgeMessenger
// npx hardhat deploy --network geth --tags L1BridgeMessenger
const deployNilGasPriceOracle: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { getNamedAccounts } = hre;
    const { deployer } = await getNamedAccounts();
    const networkName = network.name;
    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);

    // Validate configuration parameters
    if (!isValidAddress(config.nilGasPriceOracleConfig.owner)) {
        throw new Error('Invalid nilRollupOwnerAddress in config');
    }
    if (!isValidAddress(config.nilGasPriceOracleConfig.admin)) {
        throw new Error('Invalid defaultAdminAddress in config');
    }

    // Check if NilGasPriceOracle is already deployed
    if (config.nilGasPriceOracleConfig.nilGasPriceOracleProxy && isValidAddress(config.nilGasPriceOracleConfig.nilGasPriceOracleProxy)) {
        console.log(`NilGasPriceOracle already deployed at: ${config.nilGasPriceOracleConfig.nilGasPriceOracleProxy}`);
        archiveL1NetworkConfig(networkName, config);
    }

    try {
        // Deploy NilGasPriceOracle implementation
        const NilGasPriceOracle = await ethers.getContractFactory('NilGasPriceOracle');

        // proxy admin contract
        // deploys implementation contract (NilGasPriceOracle)
        // deploys ProxyContract
        // sets implementation contract address in the ProxyContract storage
        // initialize the contract
        // entire storage is owned by proxy contract
        const nilGasPriceOracleProxy = await upgrades.deployProxy(
            NilGasPriceOracle,
            [
                config.nilGasPriceOracleConfig.owner, // _owner
                config.nilGasPriceOracleConfig.admin, // _defaultAdmin
                config.nilGasPriceOracleConfig.nilGasPriceSetterAddress,
                config.nilGasPriceOracleConfig.nilGasPriceOracleMaxFeePerGas,
                config.nilGasPriceOracleConfig.nilGasPriceOracleMaxPriorityFeePerGas
            ],
            { initializer: 'initialize' },
        );

        console.log(`nilGasPriceOracleProxy deployed to: ${nilGasPriceOracleProxy.target}`);

        const nilGasPriceOracleProxyAddress = nilGasPriceOracleProxy.target;
        config.nilGasPriceOracleConfig.nilGasPriceOracleProxy = nilGasPriceOracleProxyAddress;

        // query proxyAdmin address and implementation address
        const proxyAdminAddress = await getProxyAdminAddressWithRetry(
            nilGasPriceOracleProxyAddress,
        );
        config.nilGasPriceOracleConfig.proxyAdmin = proxyAdminAddress;

        if (proxyAdminAddress === ZeroAddress) {
            throw new Error('Invalid proxy admin address');
        }

        const implementationAddress =
            await upgrades.erc1967.getImplementationAddress(
                nilGasPriceOracleProxyAddress,
            );
        config.nilGasPriceOracleConfig.nilGasPriceOracleImplementation = implementationAddress;

        if (implementationAddress === ZeroAddress) {
            throw new Error('Invalid implementation address');
        }

        // Query the proxy storage and assert if the input arguments are correctly set in the contract storage
        const nilRollup = NilGasPriceOracle.attach(nilGasPriceOracleProxyAddress);

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
                await verifyContractWithRetry(nilGasPriceOracleProxyAddress, []);
            } catch (error) {
                console.error(
                    'NilGasPriceOracleProxy Verification failed after retries:',
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

export default deployNilGasPriceOracle;
deployNilGasPriceOracle.tags = ['NilGasPriceOracle'];
