import { expect } from "chai";
import hre from "hardhat";
import "@nomicfoundation/hardhat-ethers";
import { ethers } from "hardhat";
import { deployNilContract } from "../src/deployUtil";

describe("Factory contract interaction", function () {
  let incrementerAddress: string;

  it("Should deploy Incrementer usinf Factory, increment its value and verify", async function () {
    this.timeout(120000);

    // Deploy Factory contract
    const { deployedContract: factory, contractAddress: callerAddr } = await deployNilContract("Factory");
    console.log("Factory deployed at:", callerAddr);


    // Retrieve Incrementer bytecode from Hardhat
    const Incrementer = await ethers.getContractFactory("Incrementer");
    const incrementerBytecode = Incrementer.bytecode;

    const shardId = 2; // Set shardId to 2
    const randomSalt = Math.floor(Math.random() * 10000) + 1;

    // Deploy Incrementer contract using Factory's deploy method
    await factory.deploy("Incrementer", shardId, incrementerBytecode, randomSalt);

    // Get the deployed Incrementer contract address
    incrementerAddress = await factory.getContractAddress("Incrementer");
    console.log("Incrementer deployed at:", incrementerAddress);

    // Attach to the deployed Incrementer contract
    const incrementer = Incrementer.attach(incrementerAddress);

    // Call increment function
    await incrementer.increment();

    // Check the value of Incrementer using getValue
    const value = await incrementer.getValue();
    console.log("Incrementer value:", value);

    // Expect the value to be 1 after increment
    expect(value).to.equal(1);
  });
});
