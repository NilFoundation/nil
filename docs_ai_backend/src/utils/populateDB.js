import { CheerioWebBaseLoader } from "@langchain/community/document_loaders/web/cheerio";
import { RecursiveCharacterTextSplitter } from "langchain/text_splitter";
import { DirectoryLoader } from "langchain/document_loaders/fs/directory";
import { TextLoader } from "langchain/document_loaders/fs/text";
import { OpenAIEmbeddings } from "@langchain/openai";
import { createClient } from "@libsql/client";
import { LibSQLVectorStore } from "@langchain/community/vectorstores/libsql";
import { SitemapLoader } from "@langchain/community/document_loaders/web/sitemap";
import path from "node:path";


const openAIEmbeddings = new OpenAIEmbeddings({
  model: "text-embedding-3-large",
  apiKey: process.env.OPENAI_API_KEY || "default",
  batchSize: 100
},
{
  baseUrl: 'https://api.openai.com/v1/embeddings',
  fetch: async (url, options) => {
      const result = await fetch(url, {
      method: options.method,
      body: options.body,
      headers: options.headers,
    });
      return result;
  }
}
);

const loadUrl = async (url) => {
  const loader = new CheerioWebBaseLoader(url);
  const docs = await loader.load();
  return docs;
}

export const populateDB = async () => {
  const db = createClient({
    url: `http://${process.env.DB_URL}`,
  });

  await db.execute(`
    DROP TABLE IF EXISTS EMBEDDINGS_DOCS
  `);

  await db.execute(`
    CREATE TABLE EMBEDDINGS_DOCS (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content TEXT,
    metadata TEXT,
    EMBEDDING_COLUMN F32_BLOB(3072)
  );
`);

  await db.execute(`
    CREATE INDEX idx_EMBEDDINGS_DOCS_EMBEDDING_COLUMN ON EMBEDDINGS_DOCS(libsql_vector_idx(EMBEDDING_COLUMN));
`);
  const vectorStore = new LibSQLVectorStore(openAIEmbeddings, {
    db: db,
    table: "EMBEDDINGS_DOCS",
    column: "EMBEDDING_COLUMN",
  });


  let docs = [];
  const loader = new SitemapLoader("https://docs.nil.foundation/sitemap.xml", {});
  const urls = await loader.parseSitemap();
  for (const element of urls) {
    const doc = await loadUrl(element.loc);
    docs = docs.concat(doc);
  }
  const contractsLoader = new DirectoryLoader(
    path.resolve("../../../node_modules/@nilfoundation/smart-contracts/contracts"), {
    ".sol": (path) => new TextLoader(path)
  });
  const contractDocs = await contractsLoader.load();
  const textSplitter = new RecursiveCharacterTextSplitter({
    chunkSize: 2000,
    chunkOverlap: 400,
  });

  const finalDocs = docs.concat(contractDocs);
  const splits = await textSplitter.splitDocuments(finalDocs);


  await vectorStore.addDocuments(splits);
  console.log("Documents added");
}
