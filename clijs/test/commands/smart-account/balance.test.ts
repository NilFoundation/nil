import { describe, expect } from "vitest";
import { DefualtNewSmartAccountAmount } from "../../../src/commands/smart-account/new.js";
import ConfigManager from "../../../src/common/config.js";
import { ConfigKeys } from "../../../src/common/config.js";
import { CliTest } from "../../setup.js";

// This is implementation-specific deploy cost, it may change in the future
const DeployCost = 6_379_100n;

// To run this test you need to run the nild:
// nild run --http-port 8529
// TODO: Setup nild automatically before running the tests
describe("smart-account:balance", () => {
  CliTest("runs smart-account:balance cmd", async ({ cfgPath, runCommand }) => {
    const { result } = await runCommand(["smart-account", "new"]);
    expect(result).toBeTruthy();
    const configManager = new ConfigManager(cfgPath);
    expect(result).to.equal(
      configManager.getConfigValue(ConfigKeys.NilSection, ConfigKeys.Address),
    );

    const { result: balanceResult } = await runCommand(["smart-account", "balance"]);
    expect(balanceResult).to.equal(DefualtNewSmartAccountAmount - DeployCost);
  });
});
