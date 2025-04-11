import { describe, expect } from "vitest";
import ConfigManager from "../../../src/common/config.js";
import { ConfigKeys } from "../../../src/common/config.js";
import { CliTest } from "../../setup.js";

describe("config:get", () => {
  CliTest("gets a value from the config file", async ({ cfgPath, runCommand }) => {
    const { result } = await runCommand(["config", "get", ConfigKeys.RpcEndpoint]);
    const configManager = new ConfigManager(cfgPath);

    expect(result).to.equal(
      configManager.getConfigValue(ConfigKeys.NilSection, ConfigKeys.RpcEndpoint),
    );
  });

  CliTest("returns undefined for non-existent key", async ({ runCommand }) => {
    const { result } = await runCommand(["config", "get", "non_existent_key"]);

    expect(result).toBeUndefined();
  });
});
