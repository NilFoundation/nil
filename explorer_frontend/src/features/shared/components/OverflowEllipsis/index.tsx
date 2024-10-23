import { memo } from "react";
import { useStyletron } from "styletron-react";
import type { StylesObject } from "../..";

type OverflowEllipsisProps = {
  children: string;
  canCopy?: boolean;
  charsFromTheEnd?: number;
};

const styles: StylesObject = {
  display: {
    display: "flex",
  },
  fistPartStyles: {
    whiteSpace: "nowrap",
    overflow: "hidden",
    textOverflow: "ellipsis",
    flexShrink: 1,
  },
  lastPart: {
    direction: "rtl",
  },
};

const OverflowEllipsisInternalComponent = ({
  children,
  charsFromTheEnd = 8,
}: OverflowEllipsisProps) => {
  const [css] = useStyletron();
  const firstPart = children.slice(0, -charsFromTheEnd);
  const lastPart = children.slice(-charsFromTheEnd);

  return (
    <div className={css(styles.display)}>
      <div className={css(styles.fistPartStyles)}>{firstPart}</div>
      <div className={css(styles.lastPart)}>{lastPart}</div>
    </div>
  );
};

export const OverflowEllipsis = memo(OverflowEllipsisInternalComponent);
OverflowEllipsis.displayName = "OverflowEllipsis";
