import fs from "node:fs";
import path from "node:path";
import type { ContractData, Hex } from "@nilfoundation/niljs";
import { addHexPrefix } from "@nilfoundation/niljs";
import { Args, Flags } from "@oclif/core";
import type { Abi } from "abitype";
import { BaseCommand } from "../../base.js";
import { readJsonFile } from "../../common/utils";
import { bigintFlag } from "../../types";

export default class SmartAccountDeploy extends BaseCommand {
  static override summary = "Deploy a smart contract";
  static override description =
    "Deploy the smart contract with the specified hex-bytecode from stdin or from file";

  static flags = {
    shardId: Flags.integer({
      char: "s",
      description: "Specify the shard ID to interact with",
      required: false,
      default: 1,
    }),
    salt: Flags.integer({
      char: "t",
      description: "The salt for the deploy transaction",
      required: false,
      default: 0,
    }),
    abiPath: Flags.string({
      char: "a",
      description: "The path to the ABI file",
      required: false,
    }),
    amount: bigintFlag({
      char: "m",
      description: "The amount of default tokens to send",
      required: false,
    }),
    token: bigintFlag({
      char: "c",
      description:
        'The amount of contract token to generate. This operation cannot be performed when the "no-wait" flag is set',
      required: false,
      dependsOn: ["tokenName"],
    }),
    tokenName: Flags.string({
      char: "C",
      description: "The name of the token to generate, required when the token flag is set",
      required: false,
      dependsOn: ["token"],
    }),
    noWait: Flags.boolean({
      char: "n",
      description: "Define whether the command should wait for the receipt",
      required: false,
      default: false,
    }),
    compileInput: Flags.string({
      char: "i",
      description:
        "The path to the JSON file with the compilation input. Contract will be compiled and deployed on the blockchain and the Cometa service",
      required: false,
    }),
    fee: bigintFlag({
      char: "f",
      description: "Fee credit for deployment",
      required: false,
    }),
  };

  static args = {
    filename: Args.string({
      name: "filename",
      required: false,
      description: "The path to the bytecode file",
    }),
    args: Args.string({
      name: "args",
      required: false,
      description: "Constructor arguments for the contract",
      multiple: true,
    }),
  };

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  static defaultFee = 100000000000000n;

  public async run(): Promise<Hex> {
    const { flags, args } = await this.parse(SmartAccountDeploy);

    if (flags.noWait) {
      if (flags.token) {
        this.error("The token flag cannot be set when the no-wait flag is set");
      }
      if (flags.compileInput) {
        this.error("The compileInput flag cannot be set when the no-wait flag is set");
      }
    }

    const { smartAccount } = await this.setupSmartAccount();

    let abi: Abi | undefined;

    if (flags.abiPath) {
      try {
        abi = readJsonFile<Abi>(flags.abiPath);
      } catch (e) {
        this.error(`Invalid ABI file: ${e}`);
      }
    }

    let bytecode: Hex | Uint8Array;
    let contractData = {} as ContractData;

    if (flags.compileInput) {
      const cometaClient = this.cometaClient ?? this.error("Cometa client is not initialized");
      const compileInputPath = path.resolve(flags.compileInput);
      const compileInputContent = fs.readFileSync(compileInputPath, "utf8");
      contractData = await cometaClient.compileContract(compileInputContent);
      bytecode = contractData.code;
      abi = contractData.abi as unknown as Abi;
    } else {
      const filename = args.filename;
      if (!filename) {
        this.error("at least one arg is required (the path to the bytecode file");
      }
      const fullPath = path.resolve(filename);
      bytecode = addHexPrefix(fs.readFileSync(fullPath, "utf8"));
    }

    const params = {
      shardId: flags.shardId,
      bytecode: bytecode,
      abi: abi,
      args: args.args?.split(" ") ?? [],
      salt: BigInt(flags.salt),
      value: BigInt(flags.amount ?? 0),
      feeCredit: flags.fee ?? SmartAccountDeploy.defaultFee,
    };
    const { tx, address } = await smartAccount.deployContract(params);

    if (flags.quiet) {
      this.log(address);
    } else {
      this.log("Contract address: ", address);
    }

    if (flags.noWait) {
      return address;
    }

    this.info("Waiting for the contract to be deployed...");
    // await this.waitOnTx(hash);
    await tx.wait();
    this.info("Contract successfully deployed");

    if (flags.compileInput) {
      const cometaClient = this.cometaClient ?? this.error("Cometa client is not initialized");
      await cometaClient.registerContractData(contractData, address);
    }

    if (flags.token) {
      const name =
        flags.tokenName ?? this.error("Token name is required when the token flag is set");

      let tx = await smartAccount.setTokenName(name);
      this.info("Waiting for the token name to be set...");
      await tx.wait();
      this.info("Token name successfully set");

      tx = await smartAccount.mintToken(flags.token);
      this.info("Waiting for the token to be minted...");
      await tx.wait();
      this.info("Token successfully minted");
    }

    return address;
  }
}
