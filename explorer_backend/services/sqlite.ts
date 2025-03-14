import { createHash } from "node:crypto";
import BetterSqlite3 from "better-sqlite3";

const db: BetterSqlite3.Database = new BetterSqlite3(process.env.EXPLORER_DB || "./database.db");

export { db };

db.exec(`
CREATE TABLE IF NOT EXISTS code (
    created_at TIMESTAMP,
    hash TEXT PRIMARY KEY,
    code TEXT
);
`);

const getStmt = db.prepare<string, { code: string }>("SELECT code FROM code WHERE hash = ?");

export const getCode = (hash: string): string | null => {
  return getStmt.get(hash)?.code || null;
};

export const setCode = (code: string): string => {
  const hash = createHash("sha256").update(code).digest("hex");
  const res = getStmt.get(hash);
  if (res) {
    return hash;
  }
  db.prepare("INSERT INTO code (hash, code, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)").run(
    hash,
    code,
  );
  return hash;
};
