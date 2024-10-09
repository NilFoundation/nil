import { PRIMITIVE_COLORS, SPACE } from "@nilfoundation/ui-kit";
import type { StyleObject } from "styletron-react";

const getListStyles = (showMask?: boolean, scrollable?: boolean): StyleObject => ({
  overflow: "auto",
  marginTop: SPACE[16],
  paddingRight: scrollable ? SPACE[16] : 0,
  height: "100%",
  position: "relative",
  width: "100%",
  ...(showMask ? mask : {}),
  overscrollBehavior: "contain",
});

const item: StyleObject = {
  backgroundColor: PRIMITIVE_COLORS.gray800,
  padding: SPACE[16],
  display: "grid",
  gridTemplateColumns: "70px max-content",
  gap: SPACE[12],
  marginBottom: SPACE[12],
  alignItems: "top",
  width: "100%",
};

const dummy: StyleObject = {
  height: "1px",
  visibility: "hidden",
};

const mask: StyleObject = {
  maskImage: `linear-gradient(to top, transparent, ${PRIMITIVE_COLORS.gray900} 100px)`,
  maskPosition: "top",
  maskRepeat: "no-repeat",
  maskSize: "100% 100%",
};

export const styles = {
  getListStyles,
  dummy,
  item,
};
