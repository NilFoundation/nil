import { z } from "zod";
import { client } from "../services/clickhouse";

type RawShardInfo = {
  shard_id: string;
  tx_count: string;
};

export type ShardInfo = {
  shard_id: number;
  tx_count: number;
};

export const ShardInfoSchema = z.object({
  shard_id: z.number(),
  tx_count: z.number(),
});

const mapToShardInfo = (data: RawShardInfo): ShardInfo => ({
  shard_id: Number.parseInt(data.shard_id),
  tx_count: Number.parseInt(data.tx_count),
});

export const getShardStats = async () => {
  const query = await client.query({
    query: `SELECT
    shard_id,
    sum(in_msg_num) as tx_count
    FROM blocks
    GROUP BY shard_id`,
    format: "JSON",
  });
  try {
    const res = await query.json<RawShardInfo>();
    return res.data.map(mapToShardInfo);
  } finally {
    query.close();
  }
};
