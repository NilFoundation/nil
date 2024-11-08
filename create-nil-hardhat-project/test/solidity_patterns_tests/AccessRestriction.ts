import {deployNilContract} from "../../src/deployUtil";
import {expect} from "chai";

describe("Access Restriction test", () => {
  it("positive_scenario", async () => {
    const {deployedContract: ar, contractAddress: address} = await deployNilContract("AccessRestriction");

    expect(await ar.controlValue()).to.equal(0)
    await ar.addAdmin(ar.owner())
    await ar.accessRestrictionAction()
    expect(await ar.controlValue()).to.equal(1)
  });
  it("access_restricted_scenario", async () => {
    const {deployedContract: ar} = await deployNilContract("AccessRestriction");

    expect(await ar.controlValue()).to.equal(0)
    await ar.accessRestrictionAction()
    expect(await ar.controlValue()).to.equal(0)
  });
  it("admin_excluded_from_pool", async function () {
    this.timeout(60000);

    const {deployedContract: ar} = await deployNilContract("AccessRestriction");

    await ar.addAdmin(ar.owner())
    await ar.accessRestrictionAction()
    await ar.removeAdmin(ar.owner())
    await ar.accessRestrictionAction()
    expect(await ar.controlValue()).to.equal(1)
  });
})
