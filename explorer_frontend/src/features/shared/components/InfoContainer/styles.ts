import type { StyleObject } from "styletron-react";

const heading: StyleObject = {
  display: "flex",
  justifyContent: "space-between",
  width: "100%",
};

const container: StyleObject = {
  width: "100%",
  height: "100%",
  display: "flex",
  flexDirection: "column",
};

export const styles = {
  heading,
  container,
};
