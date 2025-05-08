import * as ethers from 'ethers';

export const ZeroAddress = ethers.ZeroAddress;

// Generic function to validate addresses
export function validateAddress(address: string, fieldName: string): void {
    if (!isValidAddress(address)) {
        throw new Error(`Invalid configuration: ${fieldName} must be a valid address.`);
    }
}

export const getCheckSummedAddress = (address: string): string => {
    if (!address) {
        throw new Error(`Invalid address from configuration`);
    }
    return ethers.getAddress(address);
}

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