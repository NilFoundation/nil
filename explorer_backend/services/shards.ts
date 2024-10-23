import { fetchClients } from "../utils/fetchClients.ts";
import { fetchMasterChainInfo } from "../utils/masterChainInfo.ts";

type ShardInfo = {
  "@type": "ton.blockIdExt";
  workchain: number;
  shard: string;
  seqno: number;
  root_hash: string;
  file_hash: string;
};

type ShardsInfo = {
  "@type": "blocks.shards";
  shards: ShardInfo[];
  "@extra": string;
};

export const fetchShards = async () => {
  const [_, axiosClient] = await fetchClients();
  const currentState = await fetchMasterChainInfo(axiosClient);
  const shards = await axiosClient.get<ShardsInfo>(
    `/_api/ftfr/shards?seqno=${currentState.last.seqno}`,
  );
  return shards.data.shards;
};
