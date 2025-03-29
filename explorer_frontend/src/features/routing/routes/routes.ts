import { createHistoryRouter, createRoute } from "atomic-router";
import { createBrowserHistory } from "history";
import { addressRoute, addressTransactionsRoute } from "./addressRoute";
import { blockDetailsRoute, blockRoute } from "./blockRoute";
import { explorerRoute } from "./explorerRoute";
import { playgroundRoute, playgroundWithHashRoute } from "./playgroundRoute";
import { transactionRoute } from "./transactionRoute";
import { tutorialWithUrlStringRoute } from "./tutorialRoute";

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
    path: "/address/:address/transactions",
    route: addressTransactionsRoute,
  },
  {
    path: "/playground",
    route: playgroundRoute,
  },
  {
    path: "/playground/:snippetHash",
    route: playgroundWithHashRoute,
  },

  {
    path: "/tutorial/:urlSlug",
    route: tutorialWithUrlStringRoute,
  },

  {
    path: "/404",
    route: notFoundRoute,
  },
];

export const router = createHistoryRouter({
  routes,
  notFoundRoute,
});

export const history = createBrowserHistory();

router.setHistory(history);
