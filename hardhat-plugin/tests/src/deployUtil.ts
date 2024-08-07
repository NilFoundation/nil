import * as assert from "node:assert";
import { ethers } from "hardhat";

export async function deployNilContract(name: string, args: any[] = []) {
    const factory = await ethers.getContractFactory(name);
    assert.ok(factory.runner);
    assert.ok(factory.runner.sendTransaction);

    const deployTx = await factory.getDeployTransaction(...args);
    const sentTx = await factory.runner.sendTransaction(deployTx);
    const txReceipt = await sentTx.wait();

    if (!txReceipt || !txReceipt.contractAddress) {
        throw new Error("Contract deployment failed");
    }

    const deployedContract = factory.attach(txReceipt.contractAddress);
    return { deployedContract, contractAddress: txReceipt.contractAddress };
}
