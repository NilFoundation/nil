import { describe, expect } from "vitest";
import ConfigManager from "../../../src/common/config.js";
import { ConfigKeys } from "../../../src/common/config.js";
import { CliTest } from "../../setup.js";

describe("keygen:new", () => {
  CliTest("runs keygen:new cmd", async ({ cfgPath, runCommand }) => {
    const { result } = await runCommand(["keygen", "new"]);
    const configManager = new ConfigManager(cfgPath);
    expect(result).to.equal(
      configManager.getConfigValue(ConfigKeys.NilSection, ConfigKeys.PrivateKey),
    );
  });
});
