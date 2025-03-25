import { expect } from "chai";
import { deployNilContract } from "../src/deploy";
import type { Abi } from "abitype";
import { createSmartAccount } from "../src/smart-account";

describe("Caller and Incrementer contract interaction", () => {
  let callerAddress: string;
  let incrementerAddress: string;

  it("Should deploy Caller with shardId 2, deploy Incrementer with shardId 1, and call incrementer from caller using await", async function () {
    this.timeout(120000);

    // Deploy Caller contract
    const smartAccount = await createSmartAccount({ faucetDeposit: true });

    const AwaitJson = require("../artifacts/contracts/Await.sol/Await.json");

    const { contract: awaiter, address: callerAddr } =
      await deployNilContract(
        smartAccount,
        AwaitJson.abi as Abi,
        AwaitJson.bytecode,
        [],
        2,
        [],
      );

    callerAddress = callerAddr;
    console.log("Caller deployed at:", callerAddress);

    // Deploy Incrementer contract
    const IncrementerJson = require("../artifacts/contracts/Incrementer.sol/Incrementer.json");
    const { contract: incrementer, address: incrementerAddr } =
      await deployNilContract(
        smartAccount,
        IncrementerJson.abi as Abi,
        IncrementerJson.bytecode,
        [],
        1,
        [],
      );
    incrementerAddress = incrementerAddr;
    console.log("Incrementer deployed at:", incrementerAddress);
    // Increment the value
    const tx = await incrementer.write.increment([]);
    await tx.wait();
    // Check the value of Incrementer
    expect(await incrementer.read.getValue([])).to.equal(1);

    // Call the Caller contract's call method with the Incrementer address
    const tx2 = await awaiter.write.call([incrementerAddress]);
    await tx2.wait();

    let value = await awaiter.read.result([]);
    console.log("returned value: ", value)

    // New value should be 1
    expect(value).to.equal(1);
  });
});

