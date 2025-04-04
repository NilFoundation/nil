import { DeployFunction } from 'hardhat-deploy/types';
import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { run } from 'hardhat';
import {
    ERC20TokenContract,
    isValidAddress,
    isValidBytes32,
    L1MockConfig,
    L1MockContracts,
    L1NetworkConfig,
    loadL1MockConfig,
    loadL1NetworkConfig,
    saveL1MockConfig,
    saveL1NetworkConfig,
} from '../config/config-helper';
import { verifyContractWithRetry } from '../common/proxy-contract-utils';

export async function deployERC20TokenContracts(networkName: string, deployer: any, deploy: any): Promise<void> {
    const l1MockContracts: L1MockContracts = loadL1MockConfig(networkName);
    const erc20Tokens: ERC20TokenContract[] = l1MockContracts.tokens;

    console.log(`l1MockContracts are: ${JSON.stringify(l1MockContracts)}`);
    console.log(`l1MockContracts properties are: ${Object.keys(l1MockContracts)}`);
    console.log(`erc20Tokens are: ${JSON.stringify(erc20Tokens)} - ${erc20Tokens.length}`);

    for (const erc20Token of erc20Tokens) {
        const testERC20 = await deploy('TestERC20Token', {
            from: deployer,
            args: [erc20Token.erc20TokenInitConfig.name, erc20Token.erc20TokenInitConfig.symbol, erc20Token.erc20TokenInitConfig.decimals],
            log: true,
        });

        console.log(`ERC20Token [ name: ${erc20Token.erc20TokenInitConfig.name} - symbol: ${erc20Token.erc20TokenInitConfig.symbol} - decimal: ${erc20Token.erc20TokenInitConfig.decimals} ] deployed with address: ${testERC20.address}`);

        // Update the token's address in the config
        erc20Token.address = testERC20.address;

        // Skip verification if the network is local or anvil
        if (
            networkName !== 'local' &&
            networkName !== 'anvil' &&
            networkName !== 'geth'
        ) {
            try {
                await verifyContractWithRetry(testERC20.address, [erc20Token.erc20TokenInitConfig.name, erc20Token.erc20TokenInitConfig.symbol, erc20Token.erc20TokenInitConfig.decimals], 6);
                console.log('ERC20Token verified successfully');
            } catch (error) {
                console.error('ERC20Token Verification failed:', error);
            }
        } else {
            console.log('Skipping verification on local or anvil network');
        }
    }

    // Save the updated config
    saveL1MockConfig(networkName, l1MockContracts);
}
