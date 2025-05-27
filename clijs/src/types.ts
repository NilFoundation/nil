import type { Hex } from "@nilfoundation/niljs";
import { Args, Flags } from "@oclif/core";

export const defaultHelp = async (ctx: any) => {
  if (ctx.options.default) {
    return `${ctx.options.default.toString()}`;
  }
}

export const bigintFlag = Flags.custom<bigint>({
  parse: async (input) => BigInt(input),
  defaultHelp,
});

export const bigintArg = Args.custom<bigint>({
  parse: async (input) => BigInt(input),
  defaultHelp,
});

export const hexArg = Args.custom<Hex>({
  parse: async (input) => input as Hex,
});

export const tokenFlag = Flags.custom<{ id: Hex; amount: bigint }>({
  parse: async (input) => {
    const [tokenId, amount] = input.split("=");
    return { id: tokenId as Hex, amount: BigInt(amount) };
  },
  defaultHelp,
});
