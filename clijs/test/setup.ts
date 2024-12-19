import * as fs from "node:fs";
import ConfigManager from "../src/common/config.js";
import { ConfigKeys } from "../src/common/config.js";
import {
  CometaService,
  generateRandomPrivateKey,
  type Hex,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
} from "@nilfoundation/niljs";
import { testEnv } from "./testEnv.js";
import path from "node:path";
import os from "node:os";
import { test } from "vitest";
import { mkdtemp } from "node:fs/promises";
import { runCommand } from "@oclif/test";
import type { Errors } from "@oclif/core";

async function createTempDir() {
  const ostmpdir = os.tmpdir();
  const tmpdir = path.join(ostmpdir, "unit-test-");
  const cfgDir = await mkdtemp(tmpdir);
  const cfgPath = path.join(cfgDir, "config.ini");
  return { cfgDir, cfgPath };
}

interface CliTestFixture {
  cfgPath: string;

  runCommand: (args: string[]) => Promise<{
    error?: Error & Partial<Errors.CLIError>;
    result?: unknown;
    stderr: string;
    stdout: string;
  }>;

  cometaClient: CometaService;
  rpcClient: PublicClient;

  privateKey: Hex;
  signer: LocalECDSAKeySigner;
  wallet: WalletV1;
}

export const CliTest = test.extend<CliTestFixture>({
  cfgPath: async ({ privateKey }, use) => {
    const { cfgDir, cfgPath } = await createTempDir();
    const configManager = new ConfigManager(cfgPath);
    configManager.updateConfig(ConfigKeys.NilSection, ConfigKeys.RpcEndpoint, testEnv.endpoint);
    configManager.updateConfig(
      ConfigKeys.NilSection,
      ConfigKeys.CometaEndpoint,
      testEnv.cometaServiceEndpoint,
    );
    configManager.updateConfig(ConfigKeys.NilSection, ConfigKeys.PrivateKey, privateKey);

    await use(cfgPath);

    fs.rmSync(cfgDir, { recursive: true, force: true });
  },

  runCommand: async ({ cfgPath }, use) => {
    await use(async (cmdArgs: string[]) => {
      const args = cmdArgs.concat(["-c", cfgPath]);
      console.log("Running command:", args, "wiith root", path.join(__dirname, ".."));
      const res = await runCommand(args, {
        root: path.join(__dirname, ".."),
      });
      console.log("Command result:", res);
      return res;
    });
  },

  cometaClient: new CometaService({
    transport: new HttpTransport({
      endpoint: testEnv.cometaServiceEndpoint,
    }),
  }),

  rpcClient: new PublicClient({
    transport: new HttpTransport({
      endpoint: testEnv.endpoint,
    }),
  }),

  privateKey: generateRandomPrivateKey(),

  signer: async ({ privateKey }, use) => {
    const signer = new LocalECDSAKeySigner({
      privateKey,
    });
    await use(signer);
  },

  wallet: async ({ rpcClient, signer }, use) => {
    const wallet = new WalletV1({
      pubkey: signer.getPublicKey(),
      salt: 100n,
      shardId: 1,
      client: rpcClient,
      signer: signer,
    });
    await use(wallet);
  },
});
