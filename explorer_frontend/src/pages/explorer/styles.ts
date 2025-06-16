import { SPACE } from "@nilfoundation/ui-kit";
import type { StyleObject } from "styletron-react";
import { getMobileStyles, getTabletStyles } from "../../styleHelpers";

const container: StyleObject = {
  display: "grid",
  gridTemplateColumns: "repeat(4, 1fr)",
  gridTemplateRows: "auto 456px 320px auto",
  height: "100%",
  gap: SPACE[24],
  flexGrow: 1,
  minWidth: "0",
};

const mobileContainer: StyleObject = {
  display: "grid",
  gridTemplateColumns: "1fr",
  gridTemplateRows: "auto 520px 520px 408px 608px",
  height: "100%",
  rowGap: SPACE[24],
  flexGrow: 1,
  minWidth: "0",
};

const totalTrx: StyleObject = {
  gridColumn: "1 / 3",
  gridRow: "2 / 3",
  ...getMobileStyles({ gridColumn: "1 / 5", gridRow: "2 / 3" }),
};

const transactionHistory: StyleObject = {
  gridColumn: "3 / 5",
  gridRow: "2 / 3",
  ...getMobileStyles({ gridColumn: "1 / 5", gridRow: "3 / 4" }),
};

const shards: StyleObject = {
  gridColumn: "1 / 3",
  gridRow: "3 / 4",
  ...getMobileStyles({ gridColumn: "1 / 3", gridRow: "4 / 5" }),
  ...getTabletStyles({ overflowX: "hidden" }),
};

const blocks = {
  gridColumn: "1 / 5",
  ...getMobileStyles({ gridColumn: "1 / 5", gridRow: "5 / 6" }),
};

const heading: StyleObject = {
  gridColumn: "1 / 3",
  marginBottom: SPACE[24],
};

export const styles = {
  container,
  totalTrx,
  shards,
  blocks,
  mobileContainer,
  heading,
  transactionHistory,
};
