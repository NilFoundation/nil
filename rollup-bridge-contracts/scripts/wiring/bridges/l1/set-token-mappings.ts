import { ethers, network } from 'hardhat';
import { Contract } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadL1NetworkConfig,
    isValidAddress,
    ERC20TokenContract,
    loadL1MockConfig,
} from '../../../../deploy/config/config-helper';
const abiPath = path.join(
    __dirname,
    '../../../../artifacts/contracts/bridge/l1/interfaces/IL1ERC20Bridge.sol/IL1ERC20Bridge.json',
);
const abi = JSON.parse(fs.readFileSync(abiPath, 'utf8')).abi;

export async function setL1TokenMappings(l1TokenAddress: string, l2EnshrinedTokenAddress: string) {
    const networkName = network.name;
    const config = loadL1NetworkConfig(networkName);

    if (!isValidAddress(config.l1ERC20Bridge.l1ERC20BridgeProxy)) {
        throw new Error('Invalid l1ERC20BridgeProxy address in config');
    }

    const [signer] = await ethers.getSigners();

    const l1ERC20BridgeInstance = new ethers.Contract(
        config.l1ERC20Bridge.l1ERC20BridgeProxy,
        abi,
        signer,
    ) as Contract;

    const tx = await l1ERC20BridgeInstance.setTokenMapping(l1TokenAddress, l2EnshrinedTokenAddress);
    await tx.wait();

    console.log(`tokenMapping set for ${l1TokenAddress} -> ${l2EnshrinedTokenAddress}`);
}

export async function setTokenMappings(networkName: string) {
    // Get all tokens in l1Common in config
    const config = loadL1NetworkConfig(networkName);
    const l1MockConfig = loadL1MockConfig(networkName);

    const l1Tokens: ERC20TokenContract[] = l1MockConfig.tokens;
    const mockL2Tokens: ERC20TokenContract[] = l1MockConfig.mockL2Tokens;

    // Loop through the l1Tokens and lookup for corresponding equivalent on L2Mock token and capture the tuple
    for (const l1Token of l1Tokens) {
        const l2Token = mockL2Tokens.find(
            (mockL2Token) => mockL2Token.erc20TokenInitConfig.symbol === l1Token.erc20TokenInitConfig.symbol
        );

        if (l2Token) {
            console.log(
                `Mapping L1 Token [${l1Token.erc20TokenInitConfig.name} - ${l1Token.erc20TokenInitConfig.symbol}] to L2 Token [${l2Token.erc20TokenInitConfig.name} - ${l2Token.erc20TokenInitConfig.symbol}]`
            );

            // Call setL1TokenMappings for each tuple
            await setL1TokenMappings(l1Token.address, l2Token.address);
        } else {
            console.warn(
                `No corresponding L2 token found for L1 Token [${l1Token.erc20TokenInitConfig.name} - ${l1Token.erc20TokenInitConfig.symbol}]`
            );
        }
    }
}
