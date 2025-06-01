import { COLORS, SPACE } from "@nilfoundation/ui-kit";
import { expandProperty } from "inline-style-expand-shorthand";
import type { StyleObject } from "styletron-react";
import { getMobileStyles, getTabletStyles } from "../../../../styleHelpers";

const linkOutlineStyles = {
  borderRadius: "4px",
  ...expandProperty("padding", "4px"),
  ":focus": {
    color: COLORS.gray50,
    outline: `2px solid ${COLORS.gray200}`,
  },
};

const container: StyleObject = {
  width: "100%",
  maxWidth: "84rem",
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
  display: "grid",
  gap: "8px",
  gridTemplateColumns: "calc(66% + 8px) 33%", // 8px - width of resize handle
  gridTemplateRows: "1fr",
  height: "auto",
  ...expandProperty("padding", "16px 0"),
};

const logo = {
  display: "flex",
  alignItems: "start",
  marginRight: "auto",
  ...linkOutlineStyles,
  flexDirection: "column",
};

const navigation = {
  display: "flex",
  alignItems: "center",
  justifyContent: "flex-end",
};

const tutorialText = {
  marginTop: "4px",
  fontSize: "12px",
  color: COLORS.blue400,
  fontFamily: "Inter, sans-serif",
};

const playgroundText = {
  marginTop: "4px",
  fontSize: "12px",
  color: COLORS.gray400,
  fontFamily: "Inter, sans-serif",
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

const playgroundContainer = {
  display: "flex",
  flexDirection: "column",
  alignItems: "flex-start",
  justifyContent: "center",
  width: "100%",
  height: "100vh",
  backgroundColor: "transparent",
  ...expandProperty("paddingInline", "16px"),
  paddingBottom: "16px",
  ...getMobileStyles({
    height: "auto",
  }),
};

export const styles = {
  container,
  logo,
  navbar,
  navigation,
  navItem,
  content,
  navLink,
  playgroundText,
  tutorialText,
  playgroundContainer,
};

export const mobileContainerStyle: StyleObject = {
  ...expandProperty("padding", "16px"),
  display: "flex",
  width: "100%",
  flexDirection: "column",
  flexGrow: 1,
};

export const mobileContentStyle = {
  paddingTop: "16px",
};
