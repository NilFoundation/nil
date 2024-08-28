import { expect } from "chai";
import hre from "hardhat";
import "@nomicfoundation/hardhat-ethers";
import { deployNilContract } from "../src/deployUtil";


describe("Caller and Incrementer contract interaction", () => {
    let callerAddress: string;
    let incrementerAddress: string;

    it("Should deploy Caller with shardId 2, deploy Incrementer with shardId 1, and call incrementer from caller", async () => {
        // Set shardId to 2
        hre.config.shardId = 2;
        // Deploy Caller contract
        const { deployedContract: awaiter, contractAddress: callerAddr } = await deployNilContract("Await");
        callerAddress = callerAddr;
        console.log("Caller deployed at:", callerAddress);

        // Set shardId back to 1
        hre.config.shardId = 1;

        // Deploy Incrementer contract
        const { deployedContract: incrementer, contractAddress: incrementerAddr } = await deployNilContract("Incrementer");
        incrementerAddress = incrementerAddr;
        console.log("Incrementer deployed at:", incrementerAddress);
        // Increment the value
        await incrementer.increment();
        // Check the value of Incrementer
        expect(await incrementer.getValue()).to.equal(1);

        // Call the Caller contract's call method with the Incrementer address
        await awaiter.call(incrementerAddress);

        // TODO: Currently, there is a bug in the implementation where nil.js is not waiting for all receipts.
        // This is a temporary workaround to ensure that the necessary receipts are processed before continuing
        // Once the fix is merged, this workaround can be removed
        await new Promise(resolve => setTimeout(resolve, 3000));

        let value = await awaiter.result();
        console.log("returned value: ", value)

        // New value should be 1
        expect(value).to.equal(1);
    });
});