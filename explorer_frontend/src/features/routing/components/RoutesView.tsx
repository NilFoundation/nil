import { ErrorPage } from "@nilfoundation/ui-kit";
import { createRoutesView } from "atomic-router-react";
import { AddressPage } from "../../../pages/address";
import { BlockPage } from "../../../pages/block";
import { ExplorerPage } from "../../../pages/explorer";
import { PlaygroundPage } from "../../../pages/playground";
import { TransactionPage } from "../../../pages/transaction";
import { TutorialPage } from "../../../pages/tutorials/TutorialPage";
import { addressRoute, addressTransactionsRoute } from "../routes/addressRoute";
import { blockDetailsRoute, blockRoute } from "../routes/blockRoute";
import { explorerRoute } from "../routes/explorerRoute";
import { playgroundRoute, playgroundWithHashRoute } from "../routes/playgroundRoute";
import { transactionRoute } from "../routes/transactionRoute";
import { tutorialWithUrlStringRoute } from "../routes/tutorialRoute";

export const RoutesView = createRoutesView({
  routes: [
    { route: explorerRoute, view: ExplorerPage },
    { route: transactionRoute, view: TransactionPage },
    { route: blockRoute, view: BlockPage },
    { route: blockDetailsRoute, view: BlockPage },
    {
      route: addressRoute,
      view: AddressPage,
    },
    {
      route: addressTransactionsRoute,
      view: AddressPage,
    },
    {
      route: playgroundRoute,
      view: PlaygroundPage,
    },
    {
      route: playgroundWithHashRoute,
      view: PlaygroundPage,
    },
    {
      route: tutorialWithUrlStringRoute,
      view: TutorialPage,
    },
  ],
  otherwise() {
    return (
      <ErrorPage
        redirectPath="/"
        errorCode={404}
        redirectTitle="Back to explorer"
        errorDescription="This page does not exist"
      />
    );
  },
});
