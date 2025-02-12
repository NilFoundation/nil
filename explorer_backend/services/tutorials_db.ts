import Database from "better-sqlite3";
const db = new Database("./database.db");;

db.exec(
  `
        CREATE TABLE TUTORIALS_PROGRESS (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          hash TEXT,
          stage INTEGER
        )
      `,
);

const getStage =
  db.prepare<string, { stage: number }>("SELECT stage FROM TUTORIALS_PROGRESS WHERE hash = ?");

export const getProgress = (hash: string): number | null => {
  return getStage.get(hash)?.stage || null;
}

export const setProgress = (hash: string): void => {
  db.prepare("INSERT INTO USER_HASHES (hash, stage) VALUES (?, 0); ").run(hash);
};

export const updateProgress = (hash: string, newStage: number): void => {
  db.prepare("UPDATE USER_HASHES SET stage = ? WHERE hash = ?;").run(newStage, hash);
};

export const resetProgress = (hash: string): void => {
  db.prepare("UPDATE USER_HASHES SET stage = 0 WHERE hash = ?;").run(hash);
};