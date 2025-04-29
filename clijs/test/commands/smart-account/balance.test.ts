import { describe, expect } from "vitest";
import ConfigManager from "../../../src/common/config.js";
import { ConfigKeys } from "../../../src/common/config.js";
import { CliTest } from "../../setup.js";

describe("smart-account:balance", () => {
  CliTest("runs smart-account:balance cmd", async ({ cfgPath, runCommand }) => {
    const { result } = await runCommand(["smart-account", "new"]);
    expect(result).toBeTruthy();
    const configManager = new ConfigManager(cfgPath);
    expect(result).to.equal(
      configManager.getConfigValue(ConfigKeys.NilSection, ConfigKeys.Address),
    );

    const { result: balanceResult } = await runCommand(["smart-account", "balance"]);
    expect(balanceResult).not.to.equal(0n);
  });
});
