import "dotenv/config";
import { App } from "@tinyhttp/app";
import { json } from "milliparsec";
import "isomorphic-fetch";
import { cors } from "@tinyhttp/cors";
import { config } from "./config.ts";
import { nodeHTTPRequestHandler } from "@trpc/server/adapters/node-http";
import { appRouter } from "./routes/index.ts";
import { startCacheInterval } from "./services/cache.ts";

const app = new App({
  noMatchHandler: (_, res) => void res.send("<h1>404 Not Found</h1>"),
  onError: (err, _, res) => {
    console.error(err);
    res.status(500);
    res.send("<h1>500 Internal Server Error</h1><pre></pre>");
  },
});

app.use(
  cors({
    allowedHeaders: ["Authorization", "content-type"],
  }),
);

app.use(json());

app.use("/api", async (req, res) => {
  const opts = {
    router: appRouter,
  };
  const endpoint = req.path.slice("/api".length + 1);

  await nodeHTTPRequestHandler({
    ...opts,
    req,
    res,
    path: endpoint,
  });
});

const start = async () => {
  app.listen(config.PORT, () => {}, "127.0.0.1");

  console.log("LISTENING ON PORT", config.PORT, "...");
};

start();
startCacheInterval();
