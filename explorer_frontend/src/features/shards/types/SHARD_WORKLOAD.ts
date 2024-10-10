import { COLORS } from "@nilfoundation/ui-kit";

export enum SHARD_WORKLOAD {
  low = "low",
  medium = "medium",
  high = "high",
}

export const getBackgroundBasedOnWorkload = (workload: SHARD_WORKLOAD) => {
  switch (workload) {
    case SHARD_WORKLOAD.low:
      return COLORS.green600;
    case SHARD_WORKLOAD.medium:
      return COLORS.green500;
    case SHARD_WORKLOAD.high:
      return COLORS.green400;
  }
};
