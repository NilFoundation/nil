import { z } from "zod";
import { router, publicProcedure } from "../trpc";
import { fetchAccountState } from "../services/rpc";
import { ethAddressSchema } from "../validations/AddressScheme";
import { CurrencySchema } from "../daos/transactions";
import { hexToBigInt, numberToHex } from "viem";
import { addHexPrefix } from "@nilfoundation/niljs";
import { fetchAccountMetadata } from "../daos/account";

export const accountRouter = router({
  state: publicProcedure
    .input(ethAddressSchema)
    .output(
      z.object({
        balance: z.string(),
        code: z.string(),
        isInitialized: z.boolean(),
        currencies: z.array(CurrencySchema),
        source_code: z.optional(z.record(z.string())),
        abi: z.optional(z.any()),
      }),
    )
    .query(async (opts) => {
      const { balance, isInitialized, code, currencies } = await fetchAccountState(
        opts.input as `0x${string}`,
      );
      let sourceCode: Record<string, string> = {};
      let abi: unknown = null;
      try {
        const metadata = await fetchAccountMetadata(opts.input as `0x${string}`);
        sourceCode = metadata?.source_code || {};
        abi = JSON.parse(metadata?.abi || "");
      } catch (e) {
        abi = null;
      }

      return {
        balance,
        code,
        isInitialized,
        currencies: Object.entries(currencies).map(([currency, balance]) => {
          const numCurrency = hexToBigInt(addHexPrefix(currency));
          const address = numberToHex(numCurrency, {
            size: 20,
          });
          return CurrencySchema.parse({ currency: address, balance: balance.toString(10) });
        }),
        source_code: sourceCode,
        abi,
      };
    }),
});
