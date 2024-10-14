import loadable from "@loadable/component";
import { Spinner } from "@nilfoundation/ui-kit";

export const ExplorerPage = loadable(() => import("./ExplorerPage"), {
  fallback: <Spinner />,
});
