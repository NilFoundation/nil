import {
    ERC20TokenContract,
    L1MockContracts,
    loadL1MockConfig,
    saveL1MockConfig,
} from '../config/config-helper';
import { verifyContractWithRetry } from '../common/proxy-contract-utils';

export async function deployL2MockERC20TokenContracts(networkName: string, deployer: any, deploy: any): Promise<void> {
    const l1MockContracts: L1MockContracts = loadL1MockConfig(networkName);
    const erc20Tokens: ERC20TokenContract[] = l1MockContracts.mockL2Tokens;

    for (const erc20Token of erc20Tokens) {
        const testERC20 = await deploy('TestERC20Token', {
            from: deployer,
            args: [erc20Token.erc20TokenInitConfig.name, erc20Token.erc20TokenInitConfig.symbol, erc20Token.erc20TokenInitConfig.decimals],
            log: true,
        });

        console.log(`ERC20Token [ name: ${erc20Token.erc20TokenInitConfig.name} - symbol: ${erc20Token.symbol} - decimal: ${erc20Token.decimals} ] deployed with address: ${testERC20.address}`);

        // Update the token's address in the config
        erc20Token.address = testERC20.address;

        // Skip verification if the network is local or anvil
        if (
            networkName !== 'local' &&
            networkName !== 'anvil' &&
            networkName !== 'geth'
        ) {
            try {
                await verifyContractWithRetry(testERC20.address, [erc20Token.name, erc20Token.symbol, erc20Token.decimals], 6);
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
