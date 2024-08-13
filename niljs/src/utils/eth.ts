export const convertEthToWei = (eth: number): bigint => {
  return BigInt(eth * 1e18);
};
