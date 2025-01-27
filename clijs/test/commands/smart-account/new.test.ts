import { describe, expect } from "vitest";
import ConfigManager from "../../../src/common/config.js";
import { ConfigKeys } from "../../../src/common/config.js";
import { CliTest } from "../../setup.js";

// To run this test you need to run the nild:
// nild run --http-port 8529
// TODO: Setup nild automatically before running the tests
describe("smart-account:new", () => {
  CliTest("runs smart-account:new cmd", async ({ cfgPath, runCommand }) => {
    const { result } = await runCommand(["smart-account", "new"]);
    expect(result).toBeTruthy();
    const configManager = new ConfigManager(cfgPath);
    expect(result).to.equal(
      configManager.getConfigValue(ConfigKeys.NilSection, ConfigKeys.Address),
    );
  });
});
