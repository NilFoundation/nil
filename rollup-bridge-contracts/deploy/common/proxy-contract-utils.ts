import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { DeployFunction } from 'hardhat-deploy/types';
import { ethers, network, upgrades, run } from 'hardhat';
import { sleepInMilliSeconds } from './helper-utils';
import { ZeroAddress } from '../config/config-helper';


export async function getProxyAdminAddressWithRetry(
    proxyAddress: string,
    retries: number = 10,
): Promise<string> {
    for (let i = 0; i < retries; i++) {
        const proxyAdminAddress = await upgrades.erc1967.getAdminAddress(
            proxyAddress,
        );
        if (proxyAdminAddress !== ZeroAddress) {
            return proxyAdminAddress;
        }
        console.log(
            `ProxyAdmin address is zero. Retrying... (${i + 1}/${retries})`,
        );
        await sleepInMilliSeconds(1000 * Math.pow(2, i)); // Exponential backoff delay
    }
    throw new Error('Failed to get ProxyAdmin address after multiple attempts');
}

export async function verifyContractWithRetry(
    address: string,
    constructorArguments: any[],
    retries: number = 10,
): Promise<void> {
    for (let i = 0; i < retries; i++) {
        try {
            await run('verify:verify', {
                address,
                constructorArguments,
            });
            console.log(`Contract at ${address} verified successfully`);
            return;
        } catch (error) {
            console.error(
                `Verification failed for contract at ${address}:`,
                error,
            );
            if (i < retries - 1) {
                console.log(`Retrying verification... (${i + 1}/${retries})`);
                await sleepInMilliSeconds(1000 * Math.pow(2, i)); // Exponential backoff delay
            } else {
                throw new Error(
                    `Failed to verify contract at ${address} after ${retries} attempts`,
                );
            }
        }
    }
}

export async function getImplementationAddress(proxyAddress: string): Promise<string> {
    return await upgrades.erc1967.getImplementationAddress(
        proxyAddress,
    );
}
