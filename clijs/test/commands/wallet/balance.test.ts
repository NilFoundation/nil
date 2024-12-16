import { describe, expect } from "vitest";
import ConfigManager from "../../../src/common/config.js";
import { ConfigKeys } from "../../../src/common/config.js";
import { CliTest } from "../../setup.js";
import { DefualtNewWalletAmount } from "../../../src/commands/wallet/new.js";

// This is implementation-specific deploy cost, it may change in the future
const DeployCost = 638_920n;

// To run this test you need to run the nild:
// nild run --http-port 8529
// TODO: Setup nild automatically before running the tests
describe("wallet:balance", () => {
  CliTest("runs wallet:balance cmd", async ({ cfgPath, runCommand }) => {
    const { result } = await runCommand(["wallet", "new"]);
    expect(result).toBeTruthy();
    const configManager = new ConfigManager(cfgPath);
    expect(result).to.equal(
      configManager.getConfigValue(ConfigKeys.NilSection, ConfigKeys.Address),
    );

    const { result: balanceResult } = await runCommand(["wallet", "balance"]);
    expect(balanceResult).to.equal(DefualtNewWalletAmount - DeployCost);
  });
});
