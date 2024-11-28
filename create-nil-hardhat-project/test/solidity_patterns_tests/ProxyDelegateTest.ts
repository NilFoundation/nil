import {deployNilContract} from "../../src/deployUtil";
import {expect} from "chai";
const {ethers} = require("hardhat");

describe("Proxy delegate pattern test", function () {
  let da: any;
  let pa: any;
  let dc: any;
  let pc: any;

  before(async function () {
    const {deployedContract: delegateContract, contractAddress: delegateAddress} = await deployNilContract("Delegate");
    const {deployedContract: proxyContract, contractAddress: proxyAddress} = await deployNilContract("Proxy", [delegateAddress]);

    pa = proxyAddress;
    da = delegateAddress;
    pc = proxyContract;
    dc = delegateContract;
  });

  it("delegate positive scenario", async function () {
    const Proxy = await ethers.getContractAt("Delegate", pa);

    await Proxy.setValue(42);
    expect(await Proxy.getValue()).to.equal(42)
  });
  it("change delegate scenario", async function () {
    const {deployedContract: delegate2Contract, contractAddress: delegate2Address} = await deployNilContract("Delegate2");
    await pc.upgradeTo(delegate2Address)
    const Proxy2 = await ethers.getContractAt("Delegate2", pa);

    await Proxy2.setValue(42);
    expect(await Proxy2.getValue()).to.equal(142)
  });
});
