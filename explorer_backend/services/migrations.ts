import { readdir } from "node:fs/promises";
import { client } from "./clickhouse";
import type { ClickHouseClient } from "@clickhouse/client";

type Migration = {
  name: string;
  sequence: number;
  handler: (client: ClickHouseClient) => Promise<void>;
};

const allMigrations: Migration[] = [];

export function createMigration(
  name: string,
  sequence: number,
  handler: (client: ClickHouseClient) => Promise<void>,
) {
  allMigrations.push({ name, sequence, handler });
}

export async function runMigrations() {
  await setupMigrations();
  for (const migration of allMigrations) {
    try {
      const isProcessed = await isProcessedMigrationPointer(migration.sequence, migration.name);
      if (isProcessed) {
        continue;
      }
      await migration.handler(client);
      console.log(`Migration ${migration.name} done`);
      await setMigrationPointer(migration.sequence, migration.name);
    } catch (e) {
      throw new Error(`Migration ${migration.name} failed: ${e}`);
    }
  }
}

const isProcessedMigrationPointer = async (sequence: number, name: string): Promise<boolean> => {
  const res = await client.query({
    query:
      "SELECT sequence FROM migrations WHERE sequence = {sequence: UInt32} AND name = {name: String}",
    query_params: {
      sequence,
      name,
    },
    format: "JSON",
  });
  const json = await res.json<{ sequence: number }>();
  return json.data.length > 0;
};

const setMigrationPointer = async (sequence: number, name: string) => {
  await client.exec({
    query: "INSERT INTO migrations (sequence, name) VALUES ({sequence: UInt32}, {name: String})",
    query_params: {
      sequence,
      name,
    },
  });
};

export const fetchAllMigrations = async (directory: string): Promise<Migration[]> => {
  const files = await readdir(directory);
  for (const file of files) {
    if (!file.endsWith(".ts")) {
      continue;
    }
    await import(`${directory}/${file}`);
  }
  allMigrations.sort((a, b) => a.sequence - b.sequence);
  return allMigrations;
};

export async function setupMigrations() {
  await client.exec({
    query: `CREATE TABLE IF NOT EXISTS migrations (
                                          sequence UInt32,
                                          name String
)  ENGINE = MergeTree() PRIMARY KEY (sequence, name)`,
  });
}
