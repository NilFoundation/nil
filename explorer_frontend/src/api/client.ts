import { createTRPCProxyClient, httpBatchLink } from "@trpc/client";
import type { AppRouter } from "@nilfoundation/dbms-app-fiddle-backend";
import { $token } from "../features/auth";

export const client = createTRPCProxyClient<AppRouter>({
  links: [
    httpBatchLink({
      url: import.meta.env.VITE_API_URL || "/api",
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
