import { expect } from "chai";
import hre from "hardhat";
import "@nomicfoundation/hardhat-ethers";
import { deployNilContract } from "../../src/deployUtil";

describe("Check Effects Interaction test", () => {
  it("positive_scenario", async () => {
    const { deployedContract: CEIChild, contractAddress: childAddress } = await deployNilContract("CheckEffectsInteractionChild");
    const { deployedContract: CEIParent } = await deployNilContract("CheckEffectsInteractionParent");

    await CEIParent.topUpBalance(5000)
    expect(await CEIParent.exampleBalance()).to.equal(5000);
    await CEIParent.checkEffectsInteraction(childAddress, 500, true)
    expect(await CEIParent.exampleBalance()).to.equal(4500);
  });
  it("balance_not_changed_with_error_in_child_contract", async () => {
    const { deployedContract: CEIChild, contractAddress: childAddress } = await deployNilContract("CheckEffectsInteractionChild");
    const { deployedContract: CEIParent } = await deployNilContract("CheckEffectsInteractionParent");

    await CEIParent.topUpBalance(5000)
    expect(await CEIParent.exampleBalance()).to.equal(5000);
    await CEIParent.checkEffectsInteraction(childAddress, 500, false)
    expect(await CEIParent.exampleBalance()).to.equal(5000);
  });
  it("check_protects_contract_with_insufficient_balance", async () => {
    const { deployedContract: CEIChild, contractAddress: childAddress } = await deployNilContract("CheckEffectsInteractionChild");
    const { deployedContract: CEIParent } = await deployNilContract("CheckEffectsInteractionParent");

    await CEIParent.topUpBalance(5000)
    expect(await CEIParent.exampleBalance()).to.equal(5000);
    await CEIParent.checkEffectsInteraction(childAddress, 5500, true)
    expect(await CEIParent.exampleBalance()).to.equal(5000);
  });
});
