import { createTRPCProxyClient, httpBatchLink } from "@trpc/client";
import type { AppRouter } from "@nilfoundation/dbms-app-fiddle-backend";
import { $token } from "../features/auth";
import { getRuntimeConfigOrThrow } from "../features/runtime-config";

export const client = createTRPCProxyClient<AppRouter>({
  links: [
    httpBatchLink({
      url: getRuntimeConfigOrThrow().API_URL || "/api",
      headers: async () => {
        const token = $token.getState();
        if (!token) return {};
        return {
          Authorization: `Bearer ${token}`,
        };
      },
    }),
  ],
});
