import type { UTCTimestamp } from "lightweight-charts";

export const getUTCTimestamp = (dateTimestamp: number): UTCTimestamp => {
  return Math.trunc(dateTimestamp * 1000) as UTCTimestamp;
};
