import { createHash } from "node:crypto";
import sqlite3 from "node-sqlite3-wasm";

const db = new sqlite3.Database(process.env.EXPLORER_DB || "./database.db");

export { db };

db.exec(`
CREATE TABLE IF NOT EXISTS code (
    created_at TIMESTAMP,
    hash TEXT PRIMARY KEY,
    code TEXT
);
`);

const getStmt = db.prepare("SELECT code FROM code WHERE hash = ?");

export const getCode = (hash: string): string | null => {
  const result = getStmt.get(hash) as { code: string } | undefined;
  return result?.code || null;
};

export const setCode = async (code: string): Promise<string> => {
  const hash = createHash("sha256").update(code).digest("hex");
  const res = await getCode(hash);
  if (res) {
    return hash;
  }
  db.prepare("INSERT INTO code (hash, code, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)").run([
    hash,
    code,
  ]);
  return hash;
};
