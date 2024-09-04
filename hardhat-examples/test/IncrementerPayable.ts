import { expect } from "chai";
import hre from "hardhat";
import "@nomicfoundation/hardhat-ethers";
import { deployNilContract } from "../src/deployUtil";


describe("Incrementer contract", () => {
    it("Should increment the value", async () => {
        const { deployedContract: incrementer } = await deployNilContract("IncrementerPayable");
        console.log("IncrementerPayable", JSON.stringify(incrementer));

        // Initial value should be 0
        expect(await incrementer.getValue()).to.equal(0);

        // Increment the value
        await incrementer.increment({ value: 1n });

        // New value should be 1
        expect(await incrementer.getValue()).to.equal(1);

        // Increment the value again
        await incrementer.increment({ value: 1n });

        // New value should be 2
        expect(await incrementer.getValue()).to.equal(2);
    });
});
