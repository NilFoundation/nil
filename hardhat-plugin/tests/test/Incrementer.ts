import * as assert from "node:assert";
import { expect } from "chai";
import hre from "hardhat";
import "@nomicfoundation/hardhat-ethers";

async function deployNilContract(name: string, args: any[] = []) {
  const factory = await hre.ethers.getContractFactory(name);
  assert.ok(factory.runner);
  assert.ok(factory.runner.sendTransaction);

  const deployTx = await factory.getDeployTransaction(...args);

  const sentTx = await factory.runner.sendTransaction(deployTx);
  const txResponse = await sentTx.wait();

  if (!txResponse || !txResponse.contractAddress) {
    throw new Error("Contract deployment failed");
  }

  return factory.attach(txResponse.contractAddress);
}

describe("Incrementer contract", () => {
  it("Should increment the value", async () => {
    const incrementer = await deployNilContract("Incrementer");
    console.log("incrementer", JSON.stringify(incrementer));

    // Initial value should be 0
    expect(await incrementer.getValue()).to.equal(0);

    // Increment the value
    await incrementer.increment();

    // New value should be 1
    expect(await incrementer.getValue()).to.equal(1);

    // Increment the value again
    await incrementer.increment();

    // New value should be 2
    expect(await incrementer.getValue()).to.equal(2);
  });
});
