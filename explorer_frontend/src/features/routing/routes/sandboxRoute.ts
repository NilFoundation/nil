import { createRoute } from "../utils/createRoute";

export const sandboxRoute = createRoute();
export const sandboxWithHashRoute = createRoute<{ snippetHash: string }>();
