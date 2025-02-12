import fs from "node:fs";
import path from "node:path";

export function readJsonFile<T>(file: string): T {
  const fullPath = path.resolve(file);
  const fileContent = fs.readFileSync(fullPath, "utf8");
  return JSON.parse(fileContent) as T;
}
