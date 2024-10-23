import { Currency } from "./Currency";
import eth from "./assets/eth.svg";
import nil from "./assets/nil.svg";
import usdt from "./assets/usdt.svg";

export const getCurrencyIcon = (name: string) => {
  switch (name) {
    case Currency.ETH:
      return eth;
    case Currency.NIL:
      return nil;
    case Currency.USDT:
      return usdt;
    default:
      return null;
  }
};

export const ethAddress = "0x1111111111111111111111111111111111112";
export const usdtAddress = "0x1111111111111111111111111111111111113";
export const btcAddress = "0x1111111111111111111111111111111111114";
export const nilAddress = "0x000100000000000000000000000000000fa00ce7";

export const getCurrencySymbolByAddress = (address: string) => {
  if (address === ethAddress) {
    return Currency.ETH;
  }
  if (address === usdtAddress) {
    return Currency.USDT;
  }
  if (address === btcAddress) {
    return Currency.BTC;
  }
  return address;
};

export const getTokenAddressBySymbol = (symbol: string) => {
  if (symbol === Currency.ETH) {
    return ethAddress;
  }

  if (symbol === Currency.USDT) {
    return usdtAddress;
  }

  if (symbol === Currency.BTC) {
    return btcAddress;
  }

  return symbol;
};
