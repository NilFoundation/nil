import { DeployFunction } from 'hardhat-deploy/types';
import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { run } from 'hardhat';
import {
    ERC20Token,
    isValidAddress,
    isValidBytes32,
    L1NetworkConfig,
    loadL1NetworkConfig,
    saveL1NetworkConfig,
} from '../config/config-helper';
import { verifyContractWithRetry } from '../common/proxy-contract-utils';

// npx hardhat deploy --network sepolia --tags ERC20TokensDeploy
// npx hardhat deploy --network anvil --tags ERC20TokensDeploy
// npx hardhat deploy --network geth --tags ERC20TokensDeploy
const deployERC20Tokens: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { deployments, getNamedAccounts, ethers, network } = hre;
    const { deploy } = deployments;
    const networkName = network.name;

    const { deployer } = await getNamedAccounts();

    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);

    const erc20Tokens: ERC20Token[] = config.l1Common.tokens;

    for (const erc20Token of erc20Tokens) {
        const testERC20 = await deploy('TestERC20Token', {
            from: deployer,
            args: [erc20Token.name, erc20Token.symbol, erc20Token.decimals],
            log: true,
        });

        console.log(`ERC2Token [ name: ${erc20Token.name} - symbol: ${erc20Token.symbol} - decimal: ${erc20Token.decimals} ]  deployed with address: ${testERC20.address}`);

        // Update the token's address in the config
        erc20Token.address = testERC20.address;

        // Save the updated config
        //saveConfig(networkName, config);

        // Skip verification if the network is local or anvil
        if (
            network.name !== 'local' &&
            network.name !== 'anvil' &&
            network.name !== 'geth'
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
    saveL1NetworkConfig(networkName, config);
};

export default deployERC20Tokens;
deployERC20Tokens.tags = ['ERC20TokensDeploy'];
