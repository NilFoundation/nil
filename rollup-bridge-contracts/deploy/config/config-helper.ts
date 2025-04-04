import fs from 'fs';
import path from 'path';
import { ethers } from 'ethers';


/**
 * L1 CONFIG SCHEMA
 */

export interface L1Config {
    networks: {
        [network: string]: L1NetworkConfig;
    };
}

export interface L1NetworkConfig {
    l1DeployerConfig: L1DeployerConfig;
    l1CommonContracts: L1CommonContracts;
    nilRollup: NilRollup;
    l1BridgeRouter: L1BridgeRouter;
    l1BridgeMessenger: L1BridgeMessenger;
    l1ERC20Bridge: L1ERC20Bridge;
    l1ETHBridge: L1ETHBridge;
    nilGasPriceOracle: NilGasPriceOracle;
}

export interface L1DeployerConfig {
    owner: string;
    admin: string;
}

export interface L1CommonContracts {
    weth: string;
}

export interface NilRollup {
    nilRollupContracts: NilRollupContracts;
    nilRollupDeployerConfig: NilRollupDeployerConfig;
    nilRollupInitConfig: NilRollupInitConfig;
}

export interface NilRollupContracts {
    proxyAdmin: string;
    nilRollupImplementation: string;
    nilRollupProxy: string;
    nilVerifier: string;
}

export interface NilRollupDeployerConfig {
    proposerAddress: string;
}

export interface NilRollupInitConfig {
    l2ChainId: number;
    genesisStateRoot: string;
}

export interface L1ERC20Bridge {
    proxyAdmin: string;
    l1ERC20BridgeProxy: string;
    l1ERC20BridgeImplementation: string;
}

export interface L1ETHBridge {
    proxyAdmin: string;
    l1ETHBridgeProxy: string;
    l1ETHBridgeImplementation: string;
}

export interface L1BridgeMessenger {
    l1BridgeMessengerContracts: L1BridgeMessengerContracts;
    l1BridgeMessengerDeployerConfig: L1BridgeMessengerDeployerConfig;
}

export interface L1BridgeMessengerContracts {
    proxyAdmin: string;
    l1BridgeMessengerProxy: string;
    l1BridgeMessengerImplementation: string;
}

export interface L1BridgeMessengerDeployerConfig {
    maxProcessingTimeInEpochSeconds: number;
}

export interface L1BridgeRouter {
    proxyAdmin: string;
    l1BridgeRouterProxy: string;
    l1BridgeRouterImplementation: string;
}

export interface NilGasPriceOracle {
    nilGasPriceOracleContracts: NilGasPriceOracleContracts;
    nilGasPriceOracleDeployerConfig: NilGasPriceOracleDeployerConfig;
}

export interface NilGasPriceOracleContracts {
    proxyAdmin: string;
    nilGasPriceOracleProxy: string;
    nilGasPriceOracleImplementation: string;

}

export interface NilGasPriceOracleDeployerConfig {
    proposerAddress: string;
    nilGasPriceOracleMaxFeePerGas: number;
    nilGasPriceOracleMaxPriorityFeePerGas: number;
}


const l1NetworkConfigFilePath = path.join(__dirname, 'l1-deployment-config.json');
const l1NetworkConfigArchiveFilePath = path.join(
    __dirname,
    'archive',
    'l1-deployment-config-archive.json',
);

// Load configuration for a specific network
export const loadL1NetworkConfig = (network: string): L1NetworkConfig => {
    const config: L1Config = JSON.parse(fs.readFileSync(l1NetworkConfigFilePath, 'utf8'));
    return config.networks[network];
};

// Save configuration for a specific network
export const saveL1NetworkConfig = (
    network: string,
    networkConfig: L1NetworkConfig,
): void => {
    const config: L1Config = JSON.parse(fs.readFileSync(l1NetworkConfigFilePath, 'utf8'));
    config.networks[network] = networkConfig;
    fs.writeFileSync(l1NetworkConfigFilePath, JSON.stringify(config, null, 2), 'utf8');
};

// Archive old configuration
export const archiveL1NetworkConfig = (
    network: string,
    networkConfig: L1NetworkConfig,
): void => {
    const archiveDir = path.dirname(l1NetworkConfigArchiveFilePath);

    console.log(`archiving L1NetworkConfig to path: ${archiveDir}`);

    // Ensure the directory exists
    if (!fs.existsSync(archiveDir)) {
        fs.mkdirSync(archiveDir, { recursive: true });
    }

    let archive: {
        networks: {
            [network: string]: (L1NetworkConfig & { timestamp: string })[];
        };
    };
    try {
        archive = JSON.parse(fs.readFileSync(l1NetworkConfigArchiveFilePath, 'utf8'));
    } catch (error) {
        archive = { networks: {} };
    }

    if (!archive.networks[network]) {
        archive.networks[network] = [];
    }

    const timestamp = new Date().toISOString();
    archive.networks[network].push({ ...networkConfig, timestamp });

    console.log(`archiving the file with content to archive-path: ${l1NetworkConfigArchiveFilePath}`)

    fs.writeFileSync(l1NetworkConfigArchiveFilePath, JSON.stringify(archive, null, 2), 'utf8');
};

/**
 * L1 MOCK CONFIG SCHEMA
 */

export interface L1MockConfig {
    networks: {
        [network: string]: L1MockContracts;
    };
}

export interface L1MockContracts {
    tokens: ERC20TokenContract[];
    mockL2Tokens: ERC20TokenContract[];
    mockL2Bridge: string;
}

export interface ERC20TokenContract {
    address: string;
    erc20TokenInitConfig: ERC20TokenInitConfig;
}

export interface ERC20TokenInitConfig {
    name: string;
    symbol: string;
    decimals: number;
}


const l1MockConfigFilePath = path.join(__dirname, 'l1-mock-config.json');
const l1MockConfigArchiveFilePath = path.join(
    __dirname,
    'archive',
    'l1-mock-config-archive.json',
);

// Load configuration for a specific network
export const loadL1MockConfig = (network: string): L1MockContracts => {
    const config: L1MockConfig = JSON.parse(fs.readFileSync(l1MockConfigFilePath, 'utf8'));
    return config.networks[network];
};

// Save configuration for a specific network
export const saveL1MockConfig = (
    network: string,
    l1MockContracts: L1MockContracts,
): void => {
    const config: L1MockConfig = JSON.parse(fs.readFileSync(l1MockConfigFilePath, 'utf8'));
    config.networks[network] = l1MockContracts;
    fs.writeFileSync(l1MockConfigFilePath, JSON.stringify(config, null, 2), 'utf8');
};

// Archive old configuration
export const archiveL1MockConfig = (
    network: string,
    l1MockConfig: L1MockConfig,
): void => {
    const archiveDir = path.dirname(l1MockConfigArchiveFilePath);

    console.log(`archiving L1MockConfig to path: ${archiveDir}`);

    // Ensure the directory exists
    if (!fs.existsSync(archiveDir)) {
        fs.mkdirSync(archiveDir, { recursive: true });
    }

    let archive: {
        networks: {
            [network: string]: (L1MockConfig & { timestamp: string })[];
        };
    };
    try {
        archive = JSON.parse(fs.readFileSync(l1MockConfigArchiveFilePath, 'utf8'));
    } catch (error) {
        archive = { networks: {} };
    }

    if (!archive.networks[network]) {
        archive.networks[network] = [];
    }

    const timestamp = new Date().toISOString();
    archive.networks[network].push({ ...l1MockConfig, timestamp });

    console.log(`archiving the file with content to archive-path: ${l1MockConfigArchiveFilePath}`)

    fs.writeFileSync(l1MockConfigArchiveFilePath, JSON.stringify(archive, null, 2), 'utf8');
};


/**
 * L2 CONFIG SCHEMA
 */

export interface L2Config {
    networks: {
        [network: string]: L2NetworkConfig;
    };
}

export interface L2CommonConfig {
    owner: string;
    admin: string;
    tokens: EnshrinedToken[];
    mockL1Bridge: string;
}

export interface EnshrinedToken {
    name: string;
    symbol: string;
    decimals: number;
    address: string;
}

export interface L2NetworkConfig {
    l2Common: L2CommonConfig;
    l2BridgeRouterConfig: L2BridgeRouterConfig;
    l2BridgeMessengerConfig: L2BridgeMessengerConfig;
    l2EnshrinedTokenBridge: L2EnshrinedTokenBridgeConfig;
    l2ETHBridgeConfig: L2ETHBridgeConfig;
}

export interface L2EnshrinedTokenBridgeConfig {
    proxyAdmin: string;
    l2EnshrinedTokenBridgeProxy: string;
    l2EnsrhinedTokenBridgeImplementation: string;
}

export interface L2ETHBridgeConfig {
    proxyAdmin: string;
    l2ETHBridgeProxy: string;
    l2ETHBridgeImplementation: string;
}

export interface L2BridgeMessengerConfig {
    proxyAdmin: string;
    l2BridgeMessengerProxy: string;
    l2ridgeMessengerImplementation: string;
}

export interface L2BridgeRouterConfig {
    proxyAdmin: string;
    l2BridgeRouterProxy: string;
    l2BridgeRouterImplementation: string;
}

const nilNetworkConfigFilePath = path.join(__dirname, 'nil-deployment-config.json');
const nilNetworkConfigArchiveFilePath = path.join(
    __dirname,
    'archive',
    'nil-deployment-config-archive.json',
);

// Load configuration for a specific network
export const loadNilNetworkConfig = (network: string): L2NetworkConfig => {
    const config: L2Config = JSON.parse(fs.readFileSync(nilNetworkConfigFilePath, 'utf8'));
    return config.networks[network];
};

// Save configuration for a specific network
export const saveNilNetworkConfig = (
    network: string,
    networkConfig: L2NetworkConfig,
): void => {
    const config: L2Config = JSON.parse(fs.readFileSync(nilNetworkConfigFilePath, 'utf8'));
    config.networks[network] = networkConfig;
    fs.writeFileSync(nilNetworkConfigFilePath, JSON.stringify(config, null, 2), 'utf8');
};

// Archive old configuration
export const nilNetworkArchiveConfig = (
    network: string,
    networkConfig: L2NetworkConfig,
): void => {
    const archiveDir = path.dirname(nilNetworkConfigArchiveFilePath);

    // Ensure the directory exists
    if (!fs.existsSync(archiveDir)) {
        fs.mkdirSync(archiveDir, { recursive: true });
    }

    let archive: {
        networks: {
            [network: string]: (L2NetworkConfig & { timestamp: string })[];
        };
    };
    try {
        archive = JSON.parse(fs.readFileSync(nilNetworkConfigArchiveFilePath, 'utf8'));
    } catch (error) {
        archive = { networks: {} };
    }

    if (!archive.networks[network]) {
        archive.networks[network] = [];
    }

    const timestamp = new Date().toISOString();
    archive.networks[network].push({ ...networkConfig, timestamp });

    fs.writeFileSync(nilNetworkConfigArchiveFilePath, JSON.stringify(archive, null, 2), 'utf8');
};


/**
 * COMMON UTILITIES
 */

export const ZeroAddress = ethers.ZeroAddress;

// Validate Ethereum address
export const isValidAddress = (address: string): boolean => {
    try {
        return (
            ethers.isAddress(address) && address === ethers.getAddress(address)
        );
    } catch {
        return false;
    }
};

// Validate bytes32 value
export const isValidBytes32 = (value: string): boolean => {
    return /^0x([A-Fa-f0-9]{64})$/.test(value);
};
