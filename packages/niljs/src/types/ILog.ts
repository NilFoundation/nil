/**
 * The log interface.
 */
type ILog = {
  address: string;
  topics: string[];
  data: string;
  blockNumber: bigint;
};

export type { ILog };
