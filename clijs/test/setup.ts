import * as fs from "node:fs";
import ConfigManager from "../src/common/config.js";
import { ConfigKeys } from "../src/common/config.js";
import { generateRandomPrivateKey } from "@nilfoundation/niljs";
import { testEnv } from "./testEnv.js";
import path from "node:path";
import os from "node:os";
import { test } from "vitest";
import { mkdtemp } from "node:fs/promises";
import { runCommand } from "@oclif/test";

async function createTempDir() {
  const ostmpdir = os.tmpdir();
  const tmpdir = path.join(ostmpdir, "unit-test-");
  const cfgDir = await mkdtemp(tmpdir);
  const cfgPath = path.join(cfgDir, "config.ini");
  return { cfgDir, cfgPath };
}

interface CliTestFixture {
  cfgPath: string;
  runCommand: (args: string[]) => Promise<{ result: unknown }>;
}

export const CliTest = test.extend<CliTestFixture>({
  cfgPath: async ({}, use) => {
    const { cfgDir, cfgPath } = await createTempDir();
    const configManager = new ConfigManager(cfgPath);
    configManager.updateConfig(ConfigKeys.NilSection, ConfigKeys.RpcEndpoint, testEnv.endpoint);
    configManager.updateConfig(
      ConfigKeys.NilSection,
      ConfigKeys.PrivateKey,
      generateRandomPrivateKey(),
    );

    await use(cfgPath);

    fs.rmSync(cfgDir, { recursive: true, force: true });
  },
  runCommand: async ({ cfgPath }, use) => {
    await use(async (cmdArgs: string[]) => {
      const args = cmdArgs.concat(["-c", cfgPath]);
      console.log("Running command:", args, "wiith root", path.join(__dirname, ".."));
      const { result } = await runCommand(args, {
        root: path.join(__dirname, ".."),
      });
      return { result };
    });
  },
});
