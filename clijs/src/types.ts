import type { Hex } from "@nilfoundation/niljs";
import { Args, Flags } from "@oclif/core";

export const bigintFlag = Flags.custom<bigint>({
  char: "m",
  description: "The amount",
  parse: async (input) => BigInt(input),
});

export const bigintArg = Args.custom<bigint>({
  parse: async (input) => BigInt(input),
});

export const hexArg = Args.custom<Hex>({
  parse: async (input) => input as Hex,
});

export const tokenFlag = Flags.custom<{ id: Hex; amount: bigint }>({
  parse: async (input) => {
    const [tokenId, amount] = input.split("=");
    return { id: tokenId as Hex, amount: BigInt(amount) };
  },
});
