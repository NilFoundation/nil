import type { ClickHouseClient } from "@clickhouse/client";
import { createMigration } from "../services/migrations";

createMigration("content", 1, async (client: ClickHouseClient) => {
  await client.exec({
    query: `
      UPDATE code 
      SET content = json_object("Code.sol", code) 
      WHERE content IS NULL;`
  })
});