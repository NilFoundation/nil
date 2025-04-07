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
    content TEXT
);
`);

const getStmt = db.prepare(`
  SELECT json_each.key AS path, json_each.value AS content
  FROM code, json_each(content) 
  WHERE hash = ?
  `
);

export const getCode = (hash: string): { path: string | null, content: string | null }[] => {
  const results = getStmt.all(hash) as { path: string, content: string }[];
  return results;
};

export const setCode = async (project: { [fileName: string]: string }): Promise<string> => {
  const projectString = JSON.stringify(project);
  const hash = createHash("sha256").update(projectString).digest("hex");
  const res = await getCode(hash);
  if (res) {
    return hash;
  }
  const keys = Object.keys(project);
  const jsonObjectArgs = keys.map(key => `'${key}', ?`).join(", ");

  const sql = `
    INSERT INTO code (hash, content, created_at)
    VALUES (?, json_object(${jsonObjectArgs}), CURRENT_TIMESTAMP)
  `;

  const params = [hash, ...keys.map(key => project[key])];

  db.prepare(sql).run(params);

  return hash;
};
