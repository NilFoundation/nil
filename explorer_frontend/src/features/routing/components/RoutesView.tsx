import { createRoutesView } from "atomic-router-react";
import { ErrorPage } from "@nilfoundation/ui-kit";
import { ExplorerPage } from "../../../pages/explorer";
import { transactionRoute } from "../routes/transactionRoute";
import { TransactionPage } from "../../../pages/transaction";
import { blockDetailsRoute, blockRoute } from "../routes/blockRoute";
import { BlockPage } from "../../../pages/block";
import { explorerRoute } from "../routes/explorerRoute";
import { addressMessagesRoute, addressRoute } from "../routes/addressRoute";
import { AddressPage } from "../../../pages/address";
import { SandboxPage } from "../../../pages/sandbox";
import { sandboxRoute, sandboxWithHashRoute } from "../routes/sandboxRoute";

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
      route: addressMessagesRoute,
      view: AddressPage,
    },
    {
      route: sandboxRoute,
      view: SandboxPage,
    },
    {
      route: sandboxWithHashRoute,
      view: SandboxPage,
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
