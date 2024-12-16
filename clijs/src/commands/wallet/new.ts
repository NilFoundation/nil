import {
  bytesToHex,
  Faucet,
  type Hex,
  LocalECDSAKeySigner,
  waitTillCompleted,
  WalletV1,
} from "@nilfoundation/niljs";
import { BaseCommand } from "../../base.js";
import { ConfigKeys } from "../../common/config.js";
import { Flags } from "@oclif/core";
import logger from "../../logger.js";

export const DefualtNewWalletAmount = 100_000_000n;

export default class WalletNew extends BaseCommand {
  static override description = "Create a new wallet";

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  static flags = {
    salt: Flags.integer({
      char: "s",
      description: "The salt for the wallet address calculation",
      required: false,
      default: 0,
    }),
    shardId: Flags.integer({
      char: "i",
      description: "Specify the shard ID(>= 1) to interact with",
      required: false,
      default: 1,
      parse: async (input) => {
        const parsed = Number.parseInt(input, 10);
        if (Number.isNaN(parsed)) {
          throw new Error("Shard ID must be a number");
        }
        if (parsed < 1) {
          throw new Error("Shard ID must be greater than or equal to 1");
        }
        return parsed;
      },
    }),
    feeCredit: Flags.integer({
      char: "f",
      description:
        "The fee credit for wallet creation. If set to 0, it will be estimated automatically",
      required: false,
      default: 0,
    }),
    amount: Flags.integer({
      char: "a",
      description:
        "The initial balance (capped at 100'000'000). The deployment fee will be subtracted from this balance",
      required: false,
      default: Number(DefualtNewWalletAmount),
    }),
  };

  public async run(): Promise<Hex> {
    const { flags } = await this.parse(WalletNew);

    if (flags.amount > DefualtNewWalletAmount) {
      logger.warn(
        `The specified balance (${flags.amount}) is greater than the limit (${DefualtNewWalletAmount}). The default value is used.`,
      );
      flags.amount = Number(DefualtNewWalletAmount);
    }

    const privateKey = this.cfg?.[ConfigKeys.PrivateKey];
    if (!privateKey) {
      throw new Error(
        "Private key not found in config. Perhaps you need to run 'keygen new' first?",
      );
    }

    const signer = new LocalECDSAKeySigner({
      privateKey: privateKey as Hex,
    });

    const pubkey = signer.getPublicKey();
    const wallet = new WalletV1({
      pubkey: pubkey,
      salt: BigInt(flags.salt),
      shardId: flags.shardId,
      client:
        this.rpcClient ??
        (() => {
          throw new Error("RPC client is not initialized");
        })(),
      signer,
    });
    const address = wallet.address;

    const faucet = new Faucet(this.rpcClient);

    logger.debug(`Withdrawing ${flags.amount} to ${address}`);
    const faucetHash = await faucet.withdrawTo(address, BigInt(flags.amount));
    await waitTillCompleted(this.rpcClient, bytesToHex(faucetHash));

    if (this.quiet) {
      await wallet.selfDeploy(true);
      this.log(address);
    } else {
      this.log("Deploying wallet...");
      const tx = await wallet.selfDeploy(true);
      this.log(`Successfully deployed wallet with tx hash: ${bytesToHex(tx)}`);
      this.log(`Wallet address: ${address}`);
    }
    this.configManager?.updateConfig(ConfigKeys.NilSection, ConfigKeys.Address, address);
    return address;
  }
}
