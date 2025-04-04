import { expect } from "chai";
import { ethers, waffle } from "hardhat";
import { Contract, Signer } from "ethers";

describe("LendingPoolFactory", function () {
  let lendingPoolFactory: Contract;
  let deployer: Signer;
  let user: Signer;
  let token: Contract;

  before(async function () {
    [deployer, user] = await ethers.getSigners();

    // Deploy LendingPoolFactory contract
    const LendingPoolFactory = await ethers.getContractFactory("LendingPoolFactory");
    lendingPoolFactory = await LendingPoolFactory.deploy();
    await lendingPoolFactory.deployed();

    // Deploy MockToken for testing
    const Token = await ethers.getContractFactory("MockToken");
    token = await Token.deploy("Test Token", "TT");
    await token.deployed();
  });

  it("should deploy LendingPool and register it with GlobalLedger", async function () {
    await lendingPoolFactory.deployLendingPool();

    // Get the deployed LendingPool address
    const lendingPoolAddress = await lendingPoolFactory.getLendingPoolAddress();

    // Check if the LendingPool is registered
    const isRegistered = await lendingPoolFactory.isLendingPoolRegistered(lendingPoolAddress);
    expect(isRegistered).to.be.true;  // LendingPool should be registered
  });
});
