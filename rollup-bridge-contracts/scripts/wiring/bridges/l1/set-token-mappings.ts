import { ethers, network } from 'hardhat';
import { Contract } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    loadL1NetworkConfig,
    isValidAddress,
    ERC20Token,
} from '../../../../deploy/config/config-helper';
const abiPath = path.join(
    __dirname,
    '../../../../artifacts/contracts/bridge/l1/interfaces/IL1ERC20Bridge.sol/IL1ERC20Bridge.json',
);
const abi = JSON.parse(fs.readFileSync(abiPath, 'utf8')).abi;

// npx hardhat run scripts/wiring/bridges/l1/set-token-mappings.ts --network geth
export async function setL1TokenMappings(l1TokenAddress: string, l2EnshrinedTokenAddress: string) {
    const networkName = network.name;
    const config = loadL1NetworkConfig(networkName);

    if (!isValidAddress(config.l1ERC20BridgeConfig.l1ERC20BridgeProxy)) {
        throw new Error('Invalid l1ERC20BridgeProxy address in config');
    }

    const [signer] = await ethers.getSigners();

    const l1ERC20BridgeInstance = new ethers.Contract(
        config.l1ERC20BridgeConfig.l1ERC20BridgeProxy,
        abi,
        signer,
    ) as Contract;

    const tx = await l1ERC20BridgeInstance.setTokenMapping(l1TokenAddress, l2EnshrinedTokenAddress);
    await tx.wait();

    console.log(`tokenMapping set for ${l1TokenAddress} -> ${l2EnshrinedTokenAddress}`);
}

async function setTokenMappings() {
    // Get all tokens in l1Common in config
    const networkName = network.name;
    const config = loadL1NetworkConfig(networkName);

    const l1Tokens: ERC20Token[] = config.l1MockContracts.tokens;
    const mockL2Tokens: ERC20Token[] = config.l1MockContracts.mockL2Tokens;

    // Loop through the l1Tokens and lookup for corresponding equivalent on L2Mock token and capture the tuple
    for (const l1Token of l1Tokens) {
        const l2Token = mockL2Tokens.find(
            (mockL2Token) => mockL2Token.symbol === l1Token.symbol
        );

        if (l2Token) {
            console.log(
                `Mapping L1 Token [${l1Token.name} - ${l1Token.symbol}] to L2 Token [${l2Token.name} - ${l2Token.symbol}]`
            );

            // Call setL1TokenMappings for each tuple
            await setL1TokenMappings(l1Token.address, l2Token.address);
        } else {
            console.warn(
                `No corresponding L2 token found for L1 Token [${l1Token.name} - ${l1Token.symbol}]`
            );
        }
    }
}

async function main() {
    await setTokenMappings();
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
