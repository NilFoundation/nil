import fs from 'fs';
import path from 'path';
import { ethers } from 'ethers';

export interface NetworkConfig {
    nilRollupImplementation: string;
    nilRollupProxy: string;
    nilVerifier: string;
    proxyAdminAddress: string;
    nilRollupOwnerAddress: string;
    defaultAdminAddress: string;
    proposerAddress: string;
    l2ChainId: number;
    genesisStateRoot: string;
}

export interface Config {
    networks: {
        [network: string]: NetworkConfig;
    };
}

const configFilePath = path.join(__dirname, 'nil-deployment-config.json');
const archiveFilePath = path.join(
    __dirname,
    'archive',
    'nil-deployment-config-archive.json',
);

// Load configuration for a specific network
export const loadConfig = (network: string): NetworkConfig => {
    const config: Config = JSON.parse(fs.readFileSync(configFilePath, 'utf8'));
    return config.networks[network];
};

// Save configuration for a specific network
export const saveConfig = (
    network: string,
    networkConfig: NetworkConfig,
): void => {
    const config: Config = JSON.parse(fs.readFileSync(configFilePath, 'utf8'));
    config.networks[network] = networkConfig;
    fs.writeFileSync(configFilePath, JSON.stringify(config, null, 2), 'utf8');
};

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

// Archive old configuration
export const archiveConfig = (
    network: string,
    networkConfig: NetworkConfig,
): void => {
    const archiveDir = path.dirname(archiveFilePath);

    // Ensure the directory exists
    if (!fs.existsSync(archiveDir)) {
        fs.mkdirSync(archiveDir, { recursive: true });
    }

    let archive: {
        networks: {
            [network: string]: (NetworkConfig & { timestamp: string })[];
        };
    };
    try {
        archive = JSON.parse(fs.readFileSync(archiveFilePath, 'utf8'));
    } catch (error) {
        archive = { networks: {} };
    }

    if (!archive.networks[network]) {
        archive.networks[network] = [];
    }

    const timestamp = new Date().toISOString();
    archive.networks[network].push({ ...networkConfig, timestamp });

    fs.writeFileSync(archiveFilePath, JSON.stringify(archive, null, 2), 'utf8');
};

export const ZeroAddress = ethers.ZeroAddress;
