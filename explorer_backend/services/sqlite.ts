import { createHash } from "node:crypto";
import sqlite3 from "node-sqlite3-wasm";
import { config } from "../config";

const db = new sqlite3.Database(config.EXPLORER_CODE_SNIPPETS_DB_PATH);

export { db };

db.exec(`
CREATE TABLE IF NOT EXISTS code (
    created_at TIMESTAMP,
    hash TEXT PRIMARY KEY,
    code TEXT,
    script TEXT,
);
`);

const getStmt = db.prepare("SELECT code, script FROM code WHERE hash = ?");

export const getCode = (hash: string): { code: string | null; script: string | null } => {
  const result = getStmt.get(hash) as { code: string, script: string } | undefined;
  return {
    code: result?.code || null,
    script: result?.script || null,
  };
};

export const setCode = async (
  { code, script }: { code: string; script: string | null }
): Promise<string> => {
  const project = [code, script].join("\r\n");
  const hash = createHash("sha256").update(project).digest("hex");
  const res = await getCode(hash);
  if (res) {
    return hash;
  }
  db.prepare("INSERT INTO code (hash, code, script, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)").run([
    hash,
    code,
    script,
  ]);
  return hash;
};
