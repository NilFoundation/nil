import { createRoute } from "../utils/createRoute";

export const addressRoute = createRoute<{ address: string }>();
export const addressMessagesRoute = createRoute<{ address: string }>();
