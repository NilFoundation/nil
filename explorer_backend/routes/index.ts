import { router } from "../trpc.ts";
import { blockRouter } from "./block.ts";
import { faucetRouter } from "./faucet.ts";
import { infoRouter } from "./info";
import { transactionsRouter } from "./transactions.ts";
import { accountRouter } from "./account.ts";
import { shardsRouter } from "./shards.ts";
import { codeRouter } from "./code.ts";

export const appRouter = router({
  block: blockRouter,
  faucet: faucetRouter,
  info: infoRouter,
  transactions: transactionsRouter,
  account: accountRouter,
  shards: shardsRouter,
  code: codeRouter,
});

export type AppRouter = typeof appRouter;
