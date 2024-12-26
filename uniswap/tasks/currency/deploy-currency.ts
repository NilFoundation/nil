import { Faucet, bytesToHex } from "@nilfoundation/niljs";
import type { Abi } from "abitype";
import { task } from "hardhat/config";
import CurrencyJson from "../../artifacts/contracts/Currency.sol/Currency.json";
import { createWallet } from "../basic/basic";
import { deployNilContract } from "../util/deploy";

task("deploy-currency")
  .addParam("amount")
  .setAction(async (taskArgs, _) => {
    const wallet = await createWallet();

    const { contract, address } = await deployNilContract(
      wallet,
      CurrencyJson.abi as Abi,
      CurrencyJson.bytecode,
      ["Currency", bytesToHex(wallet.signer.getPublicKey())],
      wallet.shardId,
      ["mintCurrency"],
    );
    console.log("Currency contract deployed at address: " + address);

    const faucet = new Faucet(wallet.client);
    await faucet.withdrawToWithRetry(address);

    // @ts-ignore
    const hash = await contract.external.mintCurrency([taskArgs.amount]);
    console.log("Minted currency with hash: " + hash);
  });
