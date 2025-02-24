import btc from "../../../public/icons/token/btc.svg";
import custom from "../../../public/icons/token/custom.svg";
import eth from "../../../public/icons/token/ethereum.svg";
import nil from "../../../public/icons/token/nil.svg";
import usdt from "../../../public/icons/token/usdt.svg";
import { Currency } from "../components/currency";

// Converts a value in Wei (bigint) to Ether (string) with 18 decimal precision
export const convertWeiToEth = (wei: bigint, decimals = 18): string => {
  const eth = Number(wei) / 1e18;
  return Number.parseFloat(eth.toFixed(decimals)).toString();
};

// Retrieve faucet addresses from environment variables, throw an error if undefined
export const ethAddress = import.meta.env.VITE_ETH_ADDRESS;
export const usdtAddress = import.meta.env.VITE_USDT_ADDRESS;
export const btcAddress = import.meta.env.VITE_BTC_ADDRESS;

if (!ethAddress) {
  throw new Error("Environment variable VITE_ETH_ADDRESS is not defined");
}

if (!usdtAddress) {
  throw new Error("Environment variable VITE_USDT_ADDRESS is not defined");
}

if (!btcAddress) {
  throw new Error("Environment variable VITE_BTC_ADDRESS is not defined");
}

// Returns the icon for the given currency name
export const getTokenIcon = (name: string) => {
  switch (name) {
    case Currency.ETH:
      return eth;
    case Currency.NIL:
      return nil;
    case "Nil":
      return nil;
    case Currency.USDT:
      return usdt;
    case Currency.BTC:
      return btc;
    default:
      return custom;
  }
};

export function getCurrencies(
  tokens: { name: string; address: string; show: boolean }[],
  onlyActive: boolean,
) {
  return tokens
    .filter((token) => !onlyActive || token.show)
    .map((token) => {
      const icon = getTokenIcon(token.name);
      return { icon, label: token.name, address: token.address };
    });
}

export function getTopupCurrencies(
  tokens: { name: string; address: string; show: boolean; topupable: boolean }[],
) {
  return tokens
    .filter((token) => token.topupable)
    .map((token) => {
      const icon = getTokenIcon(token.name);
      return { icon, label: token.name, address: token.address };
    });
}
