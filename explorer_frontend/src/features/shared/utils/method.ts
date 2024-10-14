import { processFlag } from "./flag";

export const formatMethod = (method: string, flag: number) => {
  const { isRefund, isBounce, isDeploy } = processFlag(flag);
  if (isBounce) {
    return "Bounce";
  }
  if (isDeploy) {
    return "Deploy";
  }
  if (isRefund) {
    return "Refund";
  }
  if (method === "") {
    return "Transfer";
  }
  return `0x${method.toLowerCase()}`;
};
