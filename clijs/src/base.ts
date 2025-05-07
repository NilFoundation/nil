import { Command, Flags } from "@oclif/core";

import * as os from "node:os";
import * as path from "node:path";
import {
  CometaClient,
  FaucetClient,
  type Hex,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  SmartAccountV1,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import ConfigManager, { ConfigKeys } from "./common/config.js";
import { logger } from "./logger.js";

abstract class BaseCommand extends Command {
  static baseFlags = {
    config: Flags.string({
      char: "c",
      description: "Path to the configuration ini file, default: ~/.config/nil/config.ini",
      required: false,
      parse: async (input: string) => {
        if (!input) {
          return undefined;
        }
        if (path.extname(input) !== ".ini") {
          throw new Error(
            `The configuration file must be an ".ini" file, not "${path.extname(input)}"`,
          );
        }
        return input;
      },
    }),
    logLevel: Flags.string({
      char: "l",
      description: "Log level in verbose mode",
      options: ["fatal", "error", "warn", "info", "debug", "trace"],
      required: false,
      default: "info",
    }),
    verbose: Flags.boolean({
      char: "v",
      description: "Verbose mode",
      required: false,
      default: false,
    }),
    quiet: Flags.boolean({
      char: "q",
      description: "Quiet mode (print only the result and exit)",
      required: false,
      default: false,
    }),
  };

  protected configManager?: ConfigManager;
  protected cfg?: Record<string, string>;
  protected rpcClient?: PublicClient;
  protected faucetClient?: FaucetClient;
  protected cometaClient?: CometaClient;
  protected quiet = false;

  public async init(): Promise<void> {
    await super.init();
    const { flags } = await this.parse({
      flags: this.ctor.flags,
      baseFlags: (super.ctor as typeof BaseCommand).baseFlags,
      enableJsonFlag: this.ctor.enableJsonFlag,
      args: this.ctor.args,
      strict: this.ctor.strict,
    });

    this.quiet = flags.quiet;

    if (flags.verbose) {
      logger.level = flags.logLevel;
      logger.trace("Log level set to:", flags.logLevel);
    }

    let cfgPath = flags.config;

    if (!cfgPath) {
      // Determine the path to the configuration file
      const configDir = path.join(os.homedir(), ".config", "nil");
      cfgPath = path.join(configDir, "config.ini");
    }

    logger.info(`Using configuration file: ${cfgPath}`);

    this.configManager = new ConfigManager(cfgPath);

    logger.trace("Loaded configuration:", this.configManager.loadConfig());

    const rpcEndpoint = this.configManager.getConfigValue(
      ConfigKeys.NilSection,
      ConfigKeys.RpcEndpoint,
    );
    if (rpcEndpoint) {
      this.rpcClient = new PublicClient({
        transport: new HttpTransport({
          endpoint: rpcEndpoint,
        }),
      });
    }

    const faucetEndpoint = this.configManager.getConfigValue(
      ConfigKeys.NilSection,
      ConfigKeys.FaucetEndpoint,
      rpcEndpoint,
    );
    if (faucetEndpoint) {
      this.faucetClient = new FaucetClient({
        transport: new HttpTransport({
          endpoint: faucetEndpoint,
        }),
      });
    }

    const cometaEndpoint = this.configManager.getConfigValue(
      ConfigKeys.NilSection,
      ConfigKeys.CometaEndpoint,
      rpcEndpoint,
    );
    if (cometaEndpoint) {
      this.cometaClient = new CometaClient({
        transport: new HttpTransport({
          endpoint: cometaEndpoint,
        }),
      });
    }
  }

  protected async setupSmartAccount() {
    const privateKey = this.configManager?.getConfigValue(
      ConfigKeys.NilSection,
      ConfigKeys.PrivateKey,
    ) as Hex;
    if (!privateKey) {
      this.error("Private key not found in config. Perhaps you need to run 'keygen new' first?");
    }

    const smartAccountAddress = this.configManager?.getConfigValue(
      ConfigKeys.NilSection,
      ConfigKeys.Address,
    ) as Hex;
    if (!smartAccountAddress) {
      this.error("Address not found in config. Perhaps you need to run 'smart-account new' first?");
    }

    const signer = new LocalECDSAKeySigner({
      privateKey: privateKey,
    });

    const publicKey = signer.getPublicKey();
    const smartAccount = new SmartAccountV1({
      pubkey: publicKey,
      address: smartAccountAddress,
      client:
        this.rpcClient ??
        (() => {
          throw new Error("RPC client is not initialized");
        })(),
      signer,
    });

    return { privateKey, publicKey, smartAccountAddress, smartAccount, signer };
  }

  protected async waitOnTx(hash: Hex): Promise<void> {
    const rpcClient = this.rpcClient ?? this.error("RPC client is not initialized");
    const receipt = await waitTillCompleted(rpcClient, hash);
    if (receipt.some((r) => !r.success)) {
      function bigIntReplacer(unusedKey: string, value: unknown): unknown {
        return typeof value === "bigint" ? value.toString() : value;
      }
      this.error(
        `Transaction ${hash} failed. Receipts: ${JSON.stringify(receipt, bigIntReplacer)}`,
      );
    }
  }

  protected info(message?: string, ...args: unknown[]): void {
    if (!this.quiet) {
      this.log(message, ...args);
    }
  }

  protected async catch(err: Error & { exitCode?: number }): Promise<unknown> {
    return super.catch(err);
  }

  protected async finally(err: Error | undefined): Promise<unknown> {
    return super.finally(err);
  }
}

export { BaseCommand };
