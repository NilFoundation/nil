import { Client, createClient } from "@libsql/client";

class DBClient {
  client: Client;
  constructor() {
    this.client = createClient({
      url: `https://${process.env.DB_URL}`
    });
  }

  public async createDB() {
    await this.client.execute(
      `
        CREATE TABLE TUTORIALS_PROGRESS (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          hash TEXT,
          stage INTEGER
        )
      `
    );
  }

  public async insertHash(hash: string) {
    await this.client.execute(
      `
        INSERT INTO USER_HASHES (hash, stage)
        VALUES (${hash}, 0);
      `
    )
  }

  public async updateProgress(hash: string, newStage: number) {
    await this.client.execute(
      `
        UPDATE USER_HASHES
        SET stage = ${newStage}
        WHERE hash = ${hash};
      `
    );
  }

  public async retrieveProgress(hash: string) {
    const result = await this.client.execute(
      `
        SELECT stage
        FROM USER_HASHES
        WHERE hash = ${hash}; 
      `
    );

    return result;
  }

  public async resetProgress(hash: string) {
    await this.client.execute(
      `
        UPDATE USER_HASHES
        SET stage = 0
        WHERE hash = ${hash};
      `
    );
  }

  public async removeUser(hash: string) {
    await this.client.execute(
      `
        DELETE FROM USER_HASHES
        WHERE hash = ${hash};
      `
    );
  }
}

const clientInstance = new DBClient();
export default clientInstance;