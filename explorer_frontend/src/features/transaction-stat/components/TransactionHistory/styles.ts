import { SPACE } from "@nilfoundation/ui-kit";
import { expandProperty } from "inline-style-expand-shorthand";
import type { StyleObject } from "styletron-react";

const chartContainer: StyleObject = {
  flexGrow: 1,
  height: "100%",
  display: "flex",
  justifyContent: "center",
  alignItems: "center",
  flexDirection: "column",
  position: "relative",
};

const errorViewContainer: StyleObject = {
  display: "flex",
  justifyContent: "center",
  alignItems: "center",
  height: "100%",
  width: "100%",
};

const chart = {
  ...expandProperty("padding", "0!important"),
};

const legend = {
  marginBottom: SPACE[8],
  height: "48px",
  flexShrink: 0,
};

const timeIntervalToggle = {
  marginBottom: SPACE[16],
  marginTop: SPACE[16],
};

export const styles = {
  legend,
  timeIntervalToggle,
  chartContainer,
  chart,
  errorViewContainer,
};
