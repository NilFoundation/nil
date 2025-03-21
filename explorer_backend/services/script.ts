import { createHash } from "node:crypto";
import { db } from "./sqlite";

db.exec(`
  CREATE TABLE IF NOT EXISTS script (
      created_at TIMESTAMP,
      hash TEXT PRIMARY KEY,
      script TEXT
);`);

const getStmt = db.prepare<string, { script: string }>("SELECT script FROM script WHERE hash = ?");

export const getScript = (hash: string): string | null => {
  return getStmt.get(hash)?.script || null;
};

export const setScript = (script: string): string => {
  const hash = createHash("sha256").update(script).digest("hex");
  const res = getStmt.get(hash);
  if (res) {
    return hash;
  }
  db.prepare("INSERT INTO script (hash, script, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)").run(
    hash,
    script,
  );
  return hash;
};

