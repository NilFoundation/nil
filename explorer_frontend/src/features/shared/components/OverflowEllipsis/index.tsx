import { memo } from "react";
import { useStyletron } from "styletron-react";
import type { StylesObject } from "../..";

type OverflowEllipsisProps = {
  children: string;
  canCopy?: boolean;
};

const styles: StylesObject = {
  container: {
    whiteSpace: "nowrap",
    overflow: "hidden",
    textOverflow: "ellipsis",
    maxWidth: "100%",
  },
};

const OverflowEllipsisInternalComponent = ({ children }: OverflowEllipsisProps) => {
  const [css] = useStyletron();

  return <div className={css(styles.container)}>{children}</div>;
};

export const OverflowEllipsis = memo(OverflowEllipsisInternalComponent);
OverflowEllipsis.displayName = "OverflowEllipsis";
