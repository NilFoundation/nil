import { expect } from "chai";
import hre from "hardhat";
import "@nomicfoundation/hardhat-ethers";
import { deployNilContract } from "../../src/deployUtil";

describe("State machine test", () => {
  it("state_changed_once_after_request", async () => {
    const { deployedContract: stateMachineChild, contractAddress: childAddress } = await deployNilContract("StateMachineChild");
    const { deployedContract: stateMachineParent } = await deployNilContract("StateMachineParent");

    expect(await stateMachineParent.state()).to.equal(1);
    expect(await stateMachineParent.lock()).to.equal(false);

    await stateMachineParent.makeRequest(childAddress, "makeRequest(bool)", true)

    expect(await stateMachineParent.state()).to.equal(2);
    expect(await stateMachineParent.lock()).to.equal(false);
  });
  it("state_changed_once_after_few_requests", async () => {
    const { deployedContract: stateMachineChild, contractAddress: childAddress } = await deployNilContract("StateMachineChild");
    const { deployedContract: stateMachineParent } = await deployNilContract("StateMachineParent");

    expect(await stateMachineParent.state()).to.equal(1);
    expect(await stateMachineParent.lock()).to.equal(false);

    stateMachineParent.makeRequest(childAddress, "makeRequest(bool)", true)
    await new Promise(f => setTimeout(f, 2000));
    await stateMachineParent.makeRequest(childAddress, "makeRequest(bool)", true)

    expect(await stateMachineParent.state()).to.equal(2);
    expect(await stateMachineParent.lock()).to.equal(false);
  });
  it("state_not_changed_if_during_contract_call_error_happened", async () => {

    const { deployedContract: stateMachineChild, contractAddress: childAddress } = await deployNilContract("StateMachineChild");
    const { deployedContract: stateMachineParent } = await deployNilContract("StateMachineParent");

    expect(await stateMachineParent.state()).to.equal(1);
    expect(await stateMachineParent.lock()).to.equal(false);

    await stateMachineParent.makeRequest(childAddress, "fakeFunction(bool)", true)

    expect(await stateMachineParent.state()).to.equal(1);
    expect(await stateMachineParent.lock()).to.equal(false);
  });
  it("state_not_changed_if_child_contract_not_executed", async () => {

    const { deployedContract: stateMachineChild, contractAddress: childAddress } = await deployNilContract("StateMachineChild");
    const { deployedContract: stateMachineParent } = await deployNilContract("StateMachineParent");

    expect(await stateMachineParent.state()).to.equal(1);
    expect(await stateMachineParent.lock()).to.equal(false);

    await stateMachineParent.makeRequest(childAddress, "makeRequest(bool)", false)

    expect(await stateMachineParent.state()).to.equal(1);
    expect(await stateMachineParent.lock()).to.equal(false);
  });
});




