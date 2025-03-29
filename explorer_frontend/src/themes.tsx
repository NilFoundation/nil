import { COLORS, createTheme } from "@nilfoundation/ui-kit";
import { Client as Styletron } from "styletron-engine-atomic";

export const engine = new Styletron();

export const { theme } = createTheme(engine, {
  enableDefaultFonts: true,
  overrides: {
    colors: {
      backgroundSecondary: COLORS.gray800,
      contractHeaderButtonBackgroundColor: "transparent",
      contractHeaderButtonBackgroundHoverColor: "transparent",
      tokenInputBackgroundColor: COLORS.gray700,
      tokenInputBackgroundHoverColor: COLORS.gray600,
      inputButtonAndDropdownOverrideBackgroundColor: COLORS.gray800,
      inputButtonAndDropdownOverrideBackgroundHoverColor: COLORS.gray700,
      rpcUrlBackgroundColor: COLORS.gray700,
      rpcUrlBackgroundHoverColor: COLORS.gray600,
    },
    sizes: {
      copyButton: "40px",
    },
    margins: {
      marginRightCopyButton: "0px",
    },
  },
});

export const tutorialsTheme = createTheme(engine, {
  enableDefaultFonts: true,
  overrides: {
    colors: {
      backgroundPrimary: COLORS.blue900,
      backgroundSecondary: COLORS.blue800,
      backgroundTertiary: COLORS.blue700,
      contractHeaderButtonBackgroundColor: COLORS.blue800,
      contractHeaderButtonBackgroundHoverColor: COLORS.blue700,
      inputButtonAndDropdownOverrideBackgroundColor: COLORS.blue800,
      inputButtonAndDropdownOverrideBackgroundHoverColor: COLORS.blue700,
      tokenInputBackgroundColor: COLORS.blue800,
      tokenInputBackgroundHoverColor: COLORS.blue700,
      rpcUrlBackgroundColor: COLORS.blue800,
      rpcUrlBackgroundHoverColor: COLORS.blue700,
    },
    sizes: {
      copyButton: "32px",
    },
    margins: {
      marginRightCopyButton: "4px",
    },
  },
});
