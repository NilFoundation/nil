import * as decimal from "decimal.js";
import { formatEther } from "viem";

const Decimal = decimal.Decimal;

const BASE = Decimal.pow(10, 18);

export const measure = (fee: string | bigint) => {
  if (typeof fee === "bigint") {
    return `${formatEther(fee)} ETH`;
  }
  return `${formatEther(BigInt(fee))} ETH`;
};

export const measureDecimal = (fee: string) => {
  return new Decimal(fee).div(BASE);
};

export const formatDecimal = (fee: decimal.Decimal): string => {
  return fee.toString();
};
