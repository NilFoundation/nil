import { SPACE } from "@nilfoundation/ui-kit";
import type { StyleObject } from "styletron-react";
import { expandProperty } from "inline-style-expand-shorthand";
import { getMobileStyles } from "../../../../styleHelpers";

const infoContainer: StyleObject = {
  display: "grid",
  gridTemplateColumns: "1fr 1fr",
  gridTemplateRows: "auto 1fr",
  marginTop: SPACE[16],
  marginBottom: SPACE[16],
};

const shardsContainer: StyleObject = {
  width: "100%",
  flexGrow: 1,
  paddingTop: SPACE[24],
  display: "grid",
  gap: "4px",
  gridTemplateColumns: "repeat(5, 1fr)",
  gridTemplateRows: "repeat(2, 1fr)",
  ...getMobileStyles({ gridTemplateRows: "repeat(3, 1fr)", gridTemplateColumns: "repeat(4, 1fr)" }),
};

const shard: StyleObject = {
  ...expandProperty("borderRadius", "8px"),
  display: "flex",
  justifyContent: "center",
  alignItems: "center",
};

export const styles = {
  infoContainer,
  shardsContainer,
  shard,
};
