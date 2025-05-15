import * as fs from "node:fs";
import { mkdtemp } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import {
  CometaClient,
  FaucetClient,
  type Hex,
  HttpTransport,
  LocalECDSAKeySigner,
  SmartAccountV1,
  generateRandomPrivateKey,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import { PublicClient } from "@nilfoundation/niljs";
import type { Errors } from "@oclif/core";
import { runCommand } from "@oclif/test";
import { test } from "vitest";
import ConfigManager from "../src/common/config.js";
import { ConfigKeys } from "../src/common/config.js";
import { testEnv } from "./testEnv.js";

async function createTempDir() {
  const ostmpdir = os.tmpdir();
  const tmpdir = path.join(ostmpdir, "unit-test-");
  const cfgDir = await mkdtemp(tmpdir);
  const cfgPath = path.join(cfgDir, "config.ini");
  return { cfgDir, cfgPath };
}

interface CliTestFixture {
  cfgPath: string;
  configManager: ConfigManager;

  runCommand: (args: string[]) => Promise<{
    error?: Error & Partial<Errors.CLIError>;
    result?: unknown;
    stderr: string;
    stdout: string;
  }>;

  cometaClient: CometaClient;
  faucetClient: FaucetClient;
  rpcClient: PublicClient;

  privateKey: Hex;
  signer: LocalECDSAKeySigner;
  smartAccount: SmartAccountV1;
}

export const CliTest = test.extend<CliTestFixture>({
  cfgPath: async ({ privateKey, smartAccount }, use) => {
    const { cfgDir, cfgPath } = await createTempDir();
    const configManager = new ConfigManager(cfgPath);
    configManager.updateConfig(ConfigKeys.NilSection, ConfigKeys.RpcEndpoint, testEnv.endpoint);
    configManager.updateConfig(
      ConfigKeys.NilSection,
      ConfigKeys.CometaEndpoint,
      testEnv.cometaServiceEndpoint,
    );
    configManager.updateConfig(
      ConfigKeys.NilSection,
      ConfigKeys.FaucetEndpoint,
      testEnv.faucetServiceEndpoint,
    );
    configManager.updateConfig(ConfigKeys.NilSection, ConfigKeys.PrivateKey, privateKey);
    configManager.updateConfig(ConfigKeys.NilSection, ConfigKeys.Address, smartAccount.address);

    await use(cfgPath);

    fs.rmSync(cfgDir, { recursive: true, force: true });
  },

  configManager: async ({ cfgPath }, use) => {
    const configManager = new ConfigManager(cfgPath);
    await use(configManager);
  },

  runCommand: async ({ cfgPath }, use) => {
    await use(async (cmdArgs: string[]) => {
      const args = cmdArgs.concat(["-c", cfgPath]);
      const res = await runCommand(args, {
        root: path.join(__dirname, ".."),
      });
      return res;
    });
  },

  cometaClient: new CometaClient({
    transport: new HttpTransport({
      endpoint: testEnv.cometaServiceEndpoint,
    }),
  }),

  rpcClient: new PublicClient({
    transport: new HttpTransport({
      endpoint: testEnv.endpoint,
    }),
  }),

  faucetClient: new FaucetClient({
    transport: new HttpTransport({
      endpoint: testEnv.faucetServiceEndpoint,
    }),
  }),

  // biome-ignore lint/correctness/noEmptyPattern:
  privateKey: async ({}, use) => {
    const key = generateRandomPrivateKey();
    await use(key);
  },

  signer: async ({ privateKey }, use) => {
    const signer = new LocalECDSAKeySigner({
      privateKey,
    });
    await use(signer);
  },

  smartAccount: async ({ rpcClient, faucetClient, signer }, use) => {
    const smartAccount = new SmartAccountV1({
      pubkey: signer.getPublicKey(),
      salt: 100n,
      shardId: 1,
      client: rpcClient,
      signer: signer,
    });

    const faucets = await faucetClient.getAllFaucets();
    await faucetClient.topUpAndWaitUntilCompletion(
      {
        faucetAddress: faucets.NIL,
        smartAccountAddress: smartAccount.address,
        amount: 1_000_000_000_000_000_000n,
      },
      rpcClient,
    );

    const deployTx = await smartAccount.selfDeploy(true);

    await waitTillCompleted(rpcClient, deployTx.hash);
    await use(smartAccount);
  },
});
