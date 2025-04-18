import { SPACE } from "@nilfoundation/ui-kit";
import type { StyleObject } from "styletron-react";
import { getMobileStyles, getTabletStyles } from "../../styleHelpers";

const container: StyleObject = {
  display: "grid",
  gridTemplateColumns: "repeat(3, 1fr)",
  gridTemplateRows: "auto 456px 320px auto",
  height: "100%",
  gap: SPACE[32],
  flexGrow: 1,
  minWidth: "0",
};

const mobileContainer: StyleObject = {
  display: "grid",
  gridTemplateColumns: "1fr",
  gridTemplateRows: "auto 456px 403px 540px",
  height: "100%",
  rowGap: SPACE[24],
  flexGrow: 1,
  minWidth: "0",
};

const chart: StyleObject = {
  gridColumn: "1 / 4",
  gridRow: "2 / 3",
  ...getMobileStyles({ gridRow: "2 / 3" }),
};

const shards: StyleObject = {
  gridColumn: "1 / 3",
  gridRow: "3 / 4",
  ...getMobileStyles({ gridColumn: "1 / 3", gridRow: "3 / 4" }),
  ...getTabletStyles({ overflowX: "hidden" }),
};

const blocks = {
  gridColumn: "1 / 4",
};

const heading: StyleObject = {
  gridColumn: "1 / 4",
};

export const styles = {
  container,
  chart,
  shards,
  blocks,
  mobileContainer,
  heading,
};
