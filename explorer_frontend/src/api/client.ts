import { createTRPCProxyClient, httpBatchLink } from "@trpc/client";
import type { AppRouter } from "@nilfoundation/explorer-backend";
import { getRuntimeConfigOrThrow } from "../features/runtime-config";

export const client = createTRPCProxyClient<AppRouter>({
  links: [
    httpBatchLink({
      url: getRuntimeConfigOrThrow().API_URL || "/api"
    }),
  ],
});
