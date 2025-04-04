import {
    L1NetworkConfig,
    loadL1NetworkConfig,
    saveL1NetworkConfig,
} from '../config/config-helper';

export async function deployWETHTokenContract(networkName: string, deployer: any, deploy: any): Promise<void> {

    // Skip verification if the network is local or anvil
    if (networkName == 'mainnet') {
        throw new Error(`Not Permitted to deploy Token Contracts to L1-Mainnet`);
    }

    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);

    const testWETH = await deploy('WETH', {
        from: deployer,
        args: [],
        log: true,
    });

    console.log('WETHToken deployed to:', testWETH.address);

    config.l1CommonContracts.weth = testWETH.address;

    saveL1NetworkConfig(networkName, config);
}
