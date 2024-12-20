import { createHistoryRouter, createRoute } from "atomic-router";
import { createBrowserHistory } from "history";
import { transactionRoute } from "./transactionRoute";
import { blockDetailsRoute, blockRoute } from "./blockRoute";
import { explorerRoute } from "./explorerRoute";
import { addressMessagesRoute, addressRoute } from "./addressRoute";
import { sandboxRoute, sandboxWithHashRoute } from "./sandboxRoute";

export const notFoundRoute = createRoute();

export const routes = [
  {
    path: "/",
    route: explorerRoute,
  },
  {
    path: "/tx/:hash",
    route: transactionRoute,
  },
  {
    path: "/block/:shard/:id",
    route: blockRoute,
  },
  {
    path: "/block/:shard/:id/:details",
    route: blockDetailsRoute,
  },
  {
    path: "/address/:address",
    route: addressRoute,
  },
  {
    path: "/address/:address/messages",
    route: addressMessagesRoute,
  },
  {
    path: "/sandbox",
    route: sandboxRoute,
  },
  {
    path: "/sandbox/:snippetHash",
    route: sandboxWithHashRoute,
  },
];

export const router = createHistoryRouter({
  routes,
  notFoundRoute,
});

export const history = createBrowserHistory();

router.setHistory(history);
