import type { HandlerContext } from "./context";

export async function unifiedInterceptor(method: string, params: any[], context: HandlerContext) {
  switch (method) {
    default:
      if (context.isRequest) {
        return await context.originalRequest({
          method: method,
          params: params,
        });
      }
      return await context.originalSend(method, params);
  }
}
