import type { ClickHouseClient } from "@clickhouse/client";
import { createMigration } from "../services/migrations";

createMigration("init", 0, async (client: ClickHouseClient) => {
  await client.exec({
    query: `
    CREATE VIEW if not exists blocks
AS
SELECT Id,
       hex(hash)            AS hash,
       shard_id,
       hex(PrevBlock)       AS PrevBlock,
       SmartContractsRoot,
       MessagesRoot,
       ReceiptsRoot,
       ChildBlocksRootHash,
       hex(MasterChainHash) AS MasterChainHash,
       LogsBloom,
       Timestamp
FROM block_0x23dd335a8fa9e7a2851fe06acd4e7017b9ef95e13becb3fdd5e7ebae290f40d4;
    `,
  });
});
