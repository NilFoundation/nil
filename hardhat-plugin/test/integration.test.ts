import util from "node:util";
import { assert, expect } from "chai";
import * as config from "../src/config";
import { useHardhatProject } from "./helpers/project";

describe("Test plugin's tasks", () => {
  let address = "";
  let walletAddress = "";
  useHardhatProject("incrementer");

  it("Update wallet", async function () {
    // At the start wallet address is not set
    let out = await captureStdout(async () => {
      await this.hre.run({ scope: "wallet", task: "info" });
    });
    expect(out).to.match(/address: undefined/);

    // Update wallet address
    await this.hre.run({ scope: "wallet", task: "update" });
    walletAddress = config.getValue("address") as string;
    expect(walletAddress).to.match(/0x[a-fA-F0-9]{40}/);

    // Check info print wallet address
    out = await captureStdout(async () => {
      await this.hre.run({ scope: "wallet", task: "info" });
    });
    expect(out).to.match(/address: 0x[a-fA-F0-9]{40}/);

    // Check address is not changed after update over valid wallet
    await this.hre.run({ scope: "wallet", task: "update" });
    assert.equal(walletAddress, config.getValue("address"));

    // Make force update, wallet address should be changed
    await this.hre.run({ scope: "wallet", task: "update" }, { force: true });
    assert.notEqual(walletAddress, config.getValue("address"));
    walletAddress = config.getValue("address") as string;
  });

  it("Compile contracts", async function () {
    await this.hre.run("compile");
  });

  it("Deploy contract", async function () {
    const out = await captureStdout(async () => {
      await this.hre.run(
        { scope: "ignition", task: "deploy" },
        { modulePath: "ignition/modules/Incrementer.ts", reset: true },
      );
    });
    const regex = /IncrementerModule#Incrementer - (0x[a-fA-F0-9]{40})/;
    const match = out.match(regex);
    assert.isNotNull(match, "Failed to find address after deploying contract");
    address = match[1];
  });

  it("Cometa register", async function () {
    await this.hre.run(
      { scope: "cometa", task: "register" },
      { address: address, contract: "Incrementer" },
    );
  });

  it("Cometa info", async function () {
    const out = await captureStdout(async () => {
      await this.hre.run({ scope: "cometa", task: "info" }, { address: address });
    });
    const regex = /name: '(.*)',/;
    const match = out.match(regex);
    assert.isNotNull(match, `"cometa info" has invalid output`);
    assert.equal(match[1], "contracts/Incrementer.sol:Incrementer");
  });
});

async function captureStdout(func: () => void) {
  let data = "";
  const logOld = console.log;
  console.log = function (...args) {
    data += `${util.format.apply(this, args)}\n`;
  };
  await func();
  console.log = logOld;
  return data;
}
