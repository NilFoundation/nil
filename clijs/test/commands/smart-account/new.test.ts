import { describe, expect } from "vitest";
import ConfigManager from "../../../src/common/config.js";
import { ConfigKeys } from "../../../src/common/config.js";
import { CliTest } from "../../setup.js";

describe("smart-account:new", () => {
  CliTest("runs smart-account:new cmd", async ({ cfgPath, runCommand }) => {
    const result = await runCommand(["smart-account", "new"]);
    expect(result.result).toBeTruthy();
    const configManager = new ConfigManager(cfgPath);
    expect(result.result).to.equal(
      configManager.getConfigValue(ConfigKeys.NilSection, ConfigKeys.Address),
    );
    expect(result.stdout).contains("0x");
  });
});
