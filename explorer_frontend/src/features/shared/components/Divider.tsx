import { COLORS } from "@nilfoundation/ui-kit";
import { useStyletron } from "styletron-react";

export const Divider = () => {
  const [css] = useStyletron();
  return (
    <div
      className={css({
        height: "1px",
        background: COLORS.gray800,
        width: "100%",
        gridColumn: "span 2",
      })}
    />
  );
};
