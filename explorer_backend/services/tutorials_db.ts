import { db } from "./sqlite";
import fs from "node:fs/promises";

export interface Tutorial {
  text: string;
  contracts: string;
  stage: number;
}

db.exec(
  `
    CREATE TABLE IF NOT EXISTS TUTORIALS (
      id INTEGER PRIMARY KEY ,
      text TEXT,
      contracts TEXT,
      stage INTEGER UNIQUE
    )
  `,
);

const getStmt = db.prepare<[string, string, number], Tutorial>(
  "SELECT id, text, contracts FROM TUTORIALS WHERE text = ? AND contracts = ? AND stage = ?",
);

const getByStageStmt = db.prepare<number, Tutorial>(
  "SELECT id, text, contracts FROM TUTORIALS WHERE stage = ?",
);

export const getTutorial = (stage: number): Tutorial | null => {
  return getByStageStmt.get(stage) || null;
};

export const getAllTutorials = () => {
  const tutorials = db.prepare("SELECT * FROM TUTORIALS").all();
  return tutorials;
};

export const setTutorial = (text: string, contracts: string, stage: number): Tutorial => {
  const existing = getStmt.get(text, contracts, stage);
  if (existing) {
    return existing;
  }

  const updateStmt = db.prepare("UPDATE TUTORIALS SET stage = 0 WHERE stage = ?");
  updateStmt.run(stage);

  const insertStmt = db.prepare("INSERT INTO TUTORIALS (text, contracts, stage) VALUES (?, ?, ?)");
  insertStmt.run(text, contracts, stage);
  return { text, contracts, stage };
};

export const generateTutorials = async (): Promise<void> => {
  const tutorialsSpec = JSON.parse(await fs.readFile("../tutorials/spec.json", "utf-8"));

  for (const tutorial of tutorialsSpec) {
    const text = await fs.readFile(`../tutorials/${tutorial.text}`, "utf-8");
    const contracts = await fs.readFile(`../tutorials/${tutorial.contracts}`, "utf-8");
    setTutorial(text, contracts, tutorial.stage);
  }
};
