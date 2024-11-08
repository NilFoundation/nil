import {deployNilContract} from "../../src/deployUtil";
import {expect} from "chai";

describe("Guard check test", () => {
  it("positive_scenario", async () => {
    const {deployedContract: guardCheckChild, contractAddress: childAddress} = await deployNilContract("GuardCheckChild");
    const {deployedContract: guardCheckParent} = await deployNilContract("GuardCheckParent");

    await guardCheckParent.topUpBalance(5000)
    expect(await guardCheckParent.exampleBalance()).to.equal(5000)
    await guardCheckParent.guardCheck(childAddress, 1000, 4000)
    expect(await guardCheckParent.exampleBalance()).to.equal(4000)
    expect(await guardCheckChild.executed()).to.equal(true)
  });
  it("require_failed_insufficient_balance_scenario", async () => {
    const {deployedContract: guardCheckChild, contractAddress: childAddress} = await deployNilContract("GuardCheckChild");
    const {deployedContract: guardCheckParent} = await deployNilContract("GuardCheckParent");

    await guardCheckParent.topUpBalance(5000)
    expect(await guardCheckParent.exampleBalance()).to.equal(5000)
    await guardCheckParent.guardCheck(childAddress, 6000, 0)
    expect(await guardCheckParent.exampleBalance()).to.equal(5000)
    expect(await guardCheckChild.executed()).to.equal(false)
  });
  it("revert_failed_incorrect_amount_scenario", async () => {
    const {deployedContract: guardCheckChild, contractAddress: childAddress} = await deployNilContract("GuardCheckChild");
    const {deployedContract: guardCheckParent} = await deployNilContract("GuardCheckParent");

    await guardCheckParent.topUpBalance(5000)
    expect(await guardCheckParent.exampleBalance()).to.equal(5000)
    await guardCheckParent.guardCheck(childAddress, 500, 0)
    expect(await guardCheckParent.exampleBalance()).to.equal(5000)
    expect(await guardCheckChild.executed()).to.equal(false)
  });
  it("assert_failed_child_contract_not_executed", async () => {
    const {deployedContract: guardCheckChild, contractAddress: childAddress} = await deployNilContract("GuardCheckChild");
    const {deployedContract: guardCheckParent} = await deployNilContract("GuardCheckParent");

    await guardCheckParent.topUpBalance(5000)
    expect(await guardCheckParent.exampleBalance()).to.equal(5000)
    await guardCheckParent.guardCheck(childAddress, 2000, 0)
    expect(await guardCheckParent.exampleBalance()).to.equal(5000)
    expect(await guardCheckChild.executed()).to.equal(false)
  });
})
