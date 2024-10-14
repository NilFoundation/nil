import type { StyleObject } from "styletron-react";
import { expandProperty } from "inline-style-expand-shorthand";
import { COLORS, SPACE } from "@nilfoundation/ui-kit";
import { getTabletStyles } from "../../../../styleHelpers";

const linkOutlineStyles = {
  borderRadius: "4px",
  ...expandProperty("padding", "4px"),
  ":focus": {
    color: COLORS.gray50,
    outline: `2px solid ${COLORS.gray200}`,
  },
};

const container: StyleObject = {
  color: COLORS.gray50,
  display: "flex",
  flexDirection: "column",
  alignItems: "center",
  justifyContent: "start",
  backgroundColor: "transparent",
  ...expandProperty("padding", "0 16px 16px 16px"),
};

const content = {
  display: "grid",
  gridTemplateColumns: "180px 9fr 1fr",
  ...getTabletStyles({ gridTemplateColumns: "1fr 6fr" }),
  paddingTop: SPACE[48],
  gap: SPACE[32],
  width: "100%",
};

const navbar = {
  width: "100%",
  display: "flex",
  justifyContent: "space-between",
  alignItems: "center",
  height: "auto",
  ...expandProperty("padding", "16px 0"),
};

const logo = {
  display: "flex",
  alignItems: "center",
  marginRight: "auto",
  ...linkOutlineStyles,
  marginLeft: "32px",
};

const navigation = {
  display: "flex",
  alignItems: "center",
  justifyContent: "space-between",
};

const navItem = {
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  ...expandProperty("padding", "16px 12px"),
  fontSize: "16px",
  fontWeight: 500,
  textWrap: "nowrap",
};

const navLink = {
  color: COLORS.gray200,
  ...linkOutlineStyles,
};

export const styles = {
  container,
  logo,
  navbar,
  navigation,
  navItem,
  content,
  navLink,
};

export const mobileContainerStyle: StyleObject = {
  padding: "16px",
  display: "flex",
  flexDirection: "column",
};

export const mobileContentStyle = {
  paddingTop: "16px",
};
