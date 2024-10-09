import path from "node:path";
import { fetchAllMigrations, runMigrations } from "./services/migrations";

const doMigration = async () => {
  try {
    const cwd = process.cwd();
    const migrationsDir = path.resolve(cwd, "migrations");
    await fetchAllMigrations(migrationsDir);
    await runMigrations();
    console.log("Migrations done");
    process.exit(0);
  } catch (e) {
    console.error(e);
    process.exit(1);
  }
};

doMigration();
