import { Command, Flags } from "@oclif/core";

import ConfigManager, { ConfigKeys } from "./common/config.js";
import { PublicClient, FaucetClient, CometaService, HttpTransport } from "@nilfoundation/niljs";
import logger from "./logger.js";
import * as path from "node:path";
import * as os from "node:os";

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
    quite: Flags.boolean({
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
  protected cometaClient?: CometaService;
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

    this.quiet = flags.quite;

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
    const cfg = this.configManager.loadConfig();

    logger.trace("Loaded configuration:", this.cfg);

    this.cfg = cfg.nil as Record<string, string>;

    if (this.cfg[ConfigKeys.RpcEndpoint]) {
      this.rpcClient = new PublicClient({
        transport: new HttpTransport({
          endpoint: this.cfg[ConfigKeys.RpcEndpoint],
        }),
      });
    }

    if (this.cfg[ConfigKeys.FaucetEndpoint]) {
      this.faucetClient = new FaucetClient({
        transport: new HttpTransport({
          endpoint: this.cfg[ConfigKeys.FaucetEndpoint],
        }),
      });
    }

    if (this.cfg[ConfigKeys.CometaEndpoint]) {
      this.cometaClient = new CometaService({
        transport: new HttpTransport({
          endpoint: this.cfg[ConfigKeys.CometaEndpoint],
        }),
      });
    }
  }

  protected async catch(err: Error & { exitCode?: number }): Promise<unknown> {
    return super.catch(err);
  }

  protected async finally(_: Error | undefined): Promise<unknown> {
    return super.finally(_);
  }
}

export { BaseCommand };
