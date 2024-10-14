import loadable from "@loadable/component";
import { Spinner } from "@nilfoundation/ui-kit";

export const TransactionPage = loadable(() => import("./TransactionPage"), {
  fallback: <Spinner />,
});
