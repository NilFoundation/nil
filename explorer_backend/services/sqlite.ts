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

const codeCnt = db.get(`SELECT COUNT(*) AS cnt
FROM pragma_table_info('code')
WHERE name = 'content';`);
console.log(codeCnt);
if (codeCnt.cnt == 0) {
  db.exec(`alter table code add column content TEXT`);
}

db.exec(`
  UPDATE code 
  SET content = json_object('Code.sol', code) 
  WHERE content IS NULL;
  `);

const getStmt = db.prepare(`
  SELECT content
  FROM code
  WHERE hash = ?
  `);

export const getCode = (hash: string): Record<string, string> | null => {
  const result = getStmt.get(hash) as { content: string };
  if (!result) {
    return null;
  }
  const jsonResult = JSON.parse(result.content) as Record<string, string>;
  return jsonResult;
};

export const setCode = async (project: Record<string, string>): Promise<string> => {
  const projectString = JSON.stringify(project);
  const hash = createHash("sha256").update(projectString).digest("hex");
  const res = await getCode(hash);
  if (res) {
    return hash;
  }

  const sql = `
    INSERT INTO code (hash, content, created_at)
    VALUES (?, ?, CURRENT_TIMESTAMP)
  `;

  const params = [hash, projectString];

  db.prepare(sql).run(params);

  return hash;
};
