import { type Hex, LocalECDSAKeySigner, SmartAccountV1, bytesToHex } from "@nilfoundation/niljs";
import { Flags } from "@oclif/core";
import { BaseCommand } from "../../base.js";
import { ConfigKeys } from "../../common/config.js";
import { logger } from "../../logger.js";
import { bigintFlag } from "../../types";

export const DefualtNewSmartAccountAmount = 1_000_000_000_000_000n;

export default class SmartAccountNew extends BaseCommand {
  static override description = "Create a new smart account";

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  static flags = {
    salt: Flags.integer({
      char: "s",
      description: "The salt for the smart account address calculation",
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
    feeCredit: bigintFlag({
      char: "f",
      description:
        "The fee credit for smart account creation. If set to 0, it will be estimated automatically",
      required: false,
    }),
    amount: bigintFlag({
      char: "a",
      description:
        "The initial balance (capped at 100'000'000). The deployment fee will be subtracted from this balance",
      required: false,
      default: DefualtNewSmartAccountAmount,
    }),
  };

  public async run(): Promise<Hex> {
    const { flags } = await this.parse(SmartAccountNew);

    if (flags.amount > DefualtNewSmartAccountAmount) {
      logger.warn(
        `The specified balance (${flags.amount}) is greater than the limit (${DefualtNewSmartAccountAmount}). The default value is used.`,
      );
      flags.amount = DefualtNewSmartAccountAmount;
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

    if (!this.rpcClient) {
      throw new Error("RPC client is not initialized");
    }
    if (!this.faucetClient) {
      throw new Error("Faucet client is not initialized");
    }

    const pubkey = signer.getPublicKey();
    const smartAccount = new SmartAccountV1({
      pubkey: pubkey,
      salt: BigInt(flags.salt),
      shardId: flags.shardId,
      client: this.rpcClient,
      signer,
    });
    const smartAccountAddress = smartAccount.address;

    logger.debug(`Withdrawing ${flags.amount} to ${smartAccountAddress}`);

    const faucets = await this.faucetClient.getAllFaucets();
    await this.faucetClient.topUpAndWaitUntilCompletion(
      {
        amount: flags.amount,
        smartAccountAddress: smartAccountAddress,
        faucetAddress: faucets.NIL,
      },
      this.rpcClient,
    );

    this.info("Deploying smart account...");
    const tx = await smartAccount.selfDeploy(true);
    this.info(`Successfully deployed smart account with tx hash: ${bytesToHex(tx)}`);

    if (this.quiet) {
      this.log(smartAccountAddress);
    } else {
      this.log(`SmartAccount address: ${smartAccountAddress}`);
    }
    this.configManager?.updateConfig(
      ConfigKeys.NilSection,
      ConfigKeys.Address,
      smartAccountAddress,
    );
    return smartAccountAddress;
  }
}
