import type { AxiosInstance } from "axios";

export type MasterchainInfo = {
  "@type": "blocks.masterchainInfo";
  last: {
    "@type": "ton.blockIdExt";
    workchain: number;
    shard: string;
    seqno: number;
    root_hash: string;
    file_hash: string;
  };
  state_root_hash: string;
  init: {
    "@type": "ton.blockIdExt";
    workchain: number;
    shard: string;
    seqno: number;
    root_hash: string;
    file_hash: string;
  };
  "@extra": string;
};

export const fetchMasterChainInfo = async (client: AxiosInstance): Promise<MasterchainInfo> => {
  const res = await client.get<MasterchainInfo>("/_api/ftfr/getMasterchainInfo");
  return res.data;
};
