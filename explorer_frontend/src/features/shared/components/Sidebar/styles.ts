import { PRIMITIVE_COLORS, SPACE } from "@nilfoundation/ui-kit";
import type { StyleObject } from "styletron-react";

const aside: StyleObject = {
  height: "100%",
  flex: "0 0 171px",
  display: "flex",
  flexDirection: "column",
  gap: SPACE[16],
};

const item = {
  height: "32px",
};

const list: StyleObject = {
  display: "flex",
  flexDirection: "column",
  gap: SPACE[4],
};

const link = {
  display: "flex",
  alignItems: "center",
  justifyContent: "start",
  width: "100%",
};

export const styles = {
  aside,
  list,
  link,
  item,
};

export const button = {
  color: PRIMITIVE_COLORS.gray50,
  backgroundColor: PRIMITIVE_COLORS.gray800,
  width: "32px",
  height: "32px",
};

export const backLinkStyle = {
  width: "32px",
};
