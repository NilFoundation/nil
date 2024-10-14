import { Currency } from "./Currency";
import eth from "./assets/eth.svg";
import nil from "./assets/nil.svg";
import usdc from "./assets/usdc.svg";

export const getCurrencyIcon = (name: string) => {
  switch (name) {
    case Currency.ETH:
      return eth;
    case Currency.NIL:
      return nil;
    case Currency.USDC:
      return usdc;
    default:
      return "";
  }
};

export const isBaseCurrency = (name: string) => {
  return name === Currency.ETH || name === Currency.NIL || name === Currency.USDC;
};
