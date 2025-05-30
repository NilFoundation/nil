import { bridgeETHAux } from "./bridge-eth-impl";
import {Hex} from "@nilfoundation/niljs";
import {
    loadL1NetworkConfig,
    L1NetworkConfig,
} from '../../deploy/config/config-helper';



// npx hardhat run scripts/bridge-test/bridge-eth.ts --network geth
export async function bridgeETH() {

    // Lazy import inside the function
    // @ts-ignore
    const { ethers, network } = await import('hardhat');
    const signers = await ethers.getSigners();
    const signer = signers[0]; // The main signer


    console.log(`Network name is: ${network.name}`); 
    const config: L1NetworkConfig = loadL1NetworkConfig(network.name);


    const weiAmount = config.l1TestConfig.l1ETHDepositTestConfig.amount;
    const l2DepositRecipient = config.l1TestConfig.l2DepositRecipient as Hex;  

    return await bridgeETHAux(
        network.name,
        signer,
        l2DepositRecipient,
        BigInt(weiAmount),
    )        
}


async function main() {
    await bridgeETH();
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
