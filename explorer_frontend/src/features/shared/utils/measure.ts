import { formatEther } from "viem";

export const measure = (fee: string | bigint) => {
  if (typeof fee === "bigint") {
    return `${formatEther(fee)} NIL`;
  }
  return `${formatEther(BigInt(fee))} NIL`;
};
