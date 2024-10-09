import loadable from "@loadable/component";
import { Spinner } from "@nilfoundation/ui-kit";

export const Search = loadable(() => import("./components/Search"), {
  fallback: <Spinner />,
});
