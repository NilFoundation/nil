import { DeployFunction } from 'hardhat-deploy/types';
import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { ethers, upgrades, run } from 'hardhat';
import {
    loadConfig,
    saveConfig,
    archiveConfig,
    isValidAddress,
    isValidBytes32,
    NetworkConfig,
    ZeroAddress,
} from './config/config-helper';
import { BatchInfo, proposerRoleHash } from './config/nil-types';
import { getProxyAdminAddressWithRetry, verifyContractWithRetry } from './common/proxy-contract-utils';

// npx hardhat deploy --network anvil --tags NilRollupContracts
// npx hardhat deploy --network geth --tags NilRollupContracts
// npx hardhat deploy --network sepolia --tags NilRollupContracts
const deployNilRollupContracts: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { deployments, getNamedAccounts, network } = hre;
    const { deploy } = deployments;
    const networkName = network.name;

    const { deployer } = await getNamedAccounts();

    // dummy state root place holder, this will be replaced by a real value provided by team
    // all values can be set in via cli arguments or from a json file
    const genesisStateRootConst = ethers.encodeBytes32String('dummyStateRoot');
    const config: NetworkConfig = loadConfig(networkName);

    console.log(`NetworkConfig for ${networkName} is: ${JSON.stringify(config)}`);

    //verify if the config object is not null and valid NetworkConfig
    if (!config) {
        throw new Error(`Invalid NetworkConfig for network: ${networkName}`);
    }

    // Validate configuration parameters
    if (!isValidAddress(config.nilRollupConfig.owner)) {
        throw new Error('Invalid nilRollupOwnerAddress in config');
    }
    if (!isValidAddress(config.nilRollupConfig.admin)) {
        throw new Error('Invalid defaultAdminAddress in config');
    }
    if (!isValidAddress(config.nilRollupConfig.proposerAddress)) {
        throw new Error('Invalid proposerAddress in config');
    }
    if (!isValidBytes32(config.nilRollupConfig.genesisStateRoot)) {
        throw new Error('Invalid genesisStateRoot in config');
    }

    // Check if NilVerifier is already deployed
    if (config.nilRollupConfig.nilVerifier && isValidAddress(config.nilRollupConfig.nilVerifier)) {
        console.log(`NilVerifier already deployed at: ${config.nilRollupConfig.nilVerifier}`);
        archiveConfig(networkName, config);
    }

    console.log(`deploying nilVerifier`);

    const nilVerifier = await deploy('NilVerifier', {
        from: deployer,
        args: [],
        log: true,
    });

    console.log('NilVerifier deployed to:', nilVerifier.address);
    config.nilRollupConfig.nilVerifier = nilVerifier.address;

    if (!isValidAddress(config.nilRollupConfig.nilVerifier)) {
        throw new Error('Invalid nilVerifier address in config');
    }

    const nilVerifierAddress = config.nilRollupConfig.nilVerifier;
    const l2ChainId = config.nilRollupConfig.l2ChainId;
    const proposerAddress = config.nilRollupConfig.proposerAddress;
    const ownerAddress = config.nilRollupConfig.owner;
    const adminAddress = config.nilRollupConfig.admin;

    try {
        // Deploy NilRollup implementation
        const NilRollup = await ethers.getContractFactory('NilRollup');

        const nilRollupProxy = await upgrades.deployProxy(
            NilRollup,
            [
                l2ChainId,
                ownerAddress, // _owner
                adminAddress, // _defaultAdmin
                nilVerifierAddress, // nilVerifier contract address
                proposerAddress, // proposer address
                config.nilRollupConfig.genesisStateRoot,
            ],
            { initializer: 'initialize' },
        );

        console.log(`NilRollup proxy deployed to: ${nilRollupProxy.target}`);

        const nilRollupProxyAddress = nilRollupProxy.target;
        config.nilRollupConfig.nilRollupProxy = nilRollupProxyAddress;

        // query proxyAdmin address and implementation address
        const proxyAdminAddress = await getProxyAdminAddressWithRetry(
            nilRollupProxyAddress,
        );
        console.log(
            `ProxyAdmin for proxy: ${nilRollupProxyAddress} is: ${proxyAdminAddress}`,
        );
        config.nilRollupConfig.proxyAdmin = proxyAdminAddress;

        if (proxyAdminAddress === ZeroAddress) {
            throw new Error('Invalid proxy admin address');
        }

        const implementationAddress =
            await upgrades.erc1967.getImplementationAddress(
                nilRollupProxyAddress,
            );
        console.log(
            `Implementation address for proxy: ${nilRollupProxyAddress} is: ${implementationAddress}`,
        );
        config.nilRollupConfig.nilRollupImplementation = implementationAddress;

        if (implementationAddress === ZeroAddress) {
            throw new Error('Invalid implementation address');
        }

        // Query the proxy storage and assert if the input arguments are correctly set in the contract storage
        const nilRollup = NilRollup.attach(nilRollupProxyAddress);

        const storedL2ChainId = await nilRollup.l2ChainId();
        const storedOwnerAddress = await nilRollup.owner();
        const storedAdminAddress = await nilRollup.getRoleMember(
            await nilRollup.DEFAULT_ADMIN_ROLE(),
            0,
        );

        const storedNilVerifierAddress = await nilRollup.nilVerifierAddress();
        const storedProposerAddress = await nilRollup.getRoleMember(
            proposerRoleHash,
            0,
        );
        const storedGenesisStateRoot = await nilRollup
            .batchInfoRecords('GENESIS_BATCH_INDEX')
            .then((info: BatchInfo) => info.newStateRoot);

        if (storedL2ChainId.toString() !== l2ChainId.toString()) {
            throw new Error('l2ChainId mismatch');
        }
        if (storedOwnerAddress.toLowerCase() !== ownerAddress.toLowerCase()) {
            throw new Error('ownerAddress mismatch');
        }
        if (storedAdminAddress.toLowerCase() !== adminAddress.toLowerCase()) {
            throw new Error('adminAddress mismatch');
        }
        if (
            storedNilVerifierAddress.toLowerCase() !==
            nilVerifierAddress.toLowerCase()
        ) {
            throw new Error('nilVerifierAddress mismatch');
        }
        if (
            storedProposerAddress.toLowerCase() !==
            proposerAddress.toLowerCase()
        ) {
            throw new Error('proposerAddress mismatch');
        }
        if (
            storedGenesisStateRoot.toLowerCase() !==
            config.nilRollupConfig.genesisStateRoot.toLowerCase()
        ) {
            throw new Error('genesisStateRoot mismatch');
        }

        // Save the updated config
        saveConfig(networkName, config);

        // check network and verify if its not geth or anvil
        // Skip verification if the network is local or anvil
        if (
            network.name !== 'local' &&
            network.name !== 'anvil' &&
            network.name !== 'geth'
        ) {
            try {
                await verifyContractWithRetry(nilVerifier.address, []);
            } catch (error) {
                console.error(
                    'NilVerifier Verification failed after retries:',
                    error,
                );
            }

            try {
                await verifyContractWithRetry(nilRollupProxyAddress, []);
            } catch (error) {
                console.error(
                    'NilRollup Verification failed after retries:',
                    error,
                );
            }
        } else {
            console.log('Skipping verification on local or anvil network');
        }
    } catch (error) {
        console.error('Error during deployment:', error);
        process.exit(1);
    }
};

export default deployNilRollupContracts;
deployNilRollupContracts.tags = ['NilRollupContracts'];
