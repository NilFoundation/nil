import type { Hex } from "@nilfoundation/niljs";
import { Flags } from "@oclif/core";
import { BaseCommand } from "../../base.js";
import { bigintFlag, hexArg, tokenFlag } from "../../types.js";

export default class SmartAccountSendToken extends BaseCommand {
  static override summary = "Transfer tokens to a specific address";
  static override description = "Transfer some amount of tokens to a specific address";

  static flags = {
    amount: bigintFlag({
      char: "m",
      description: "The amount of default tokens to send",
      required: false,
    }),
    noWait: Flags.boolean({
      char: "n",
      description: "Define whether the command should wait for the receipt",
      default: false,
    }),
    feeCredit: bigintFlag({
      char: "f",
      description: "The fee credit for transaction processing",
      required: false,
    }),
    tokens: tokenFlag({
      char: "t",
      description:
        "The custom tokens to transfer in as a map 'tokenId=amount', can be set multiple times",
      multiple: true,
      required: false,
    }),
  };

  static args = {
    address: hexArg({
      name: "address",
      required: true,
      description: "The address of the smart contract",
    }),
  };

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  public async run(): Promise<Hex> {
    const { flags, args } = await this.parse(SmartAccountSendToken);
    const { smartAccount } = await this.setupSmartAccount();

    const balance = await smartAccount.getBalance();
    this.info("balance", balance);

    const tx = await smartAccount.sendTransaction({
      to: args.address,
      value: flags.amount ?? 0n,
      feeCredit: flags.feeCredit,
      tokens: flags.tokens ?? [],
    });

    if (flags.quiet) {
      this.log(tx.hash);
    } else {
      this.log(`Transaction hash: ${tx.hash}`);
    }

    if (flags.noWait) {
      return tx.hash;
    }

    this.info("Waiting for the transaction to be processed...");
    await tx.wait();
    this.info("Transaction successfully processed");

    return tx.hash;
  }
}
