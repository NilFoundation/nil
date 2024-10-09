import { COLORS, LabelSmall, Tooltip as UIKitTooltip } from "@nilfoundation/ui-kit";
import { type ForwardRefRenderFunction, forwardRef } from "react";
import { formatUTCTimestamp } from "../../../shared";
import { useStyletron } from "styletron-react";
import type { PopoverOverrides } from "baseui/popover";
import type { Time } from "lightweight-charts";
import { Marker } from "./Marker";

type TooltipData = {
  tps?: string;
  time?: Time;
};

export type TooltipProps = {
  data: TooltipData;
  isOpen: boolean;
  position?: {
    left: number;
    top: number;
  };
  width?: number;
  height?: number;
};

const styles = {
  container: {
    display: "flex",
    flexDirection: "column",
    alignItems: "flex-start",
    gap: "8px",
  },
  dummy: {
    position: "absolute",
    top: 0,
    left: 0,
  },
  tpsContainer: {
    display: "flex",
    gap: "4px",
    alignItems: "center",
  },
} as const;

const RenderFunc: ForwardRefRenderFunction<HTMLDivElement, TooltipProps> = (
  { data: { tps, time }, isOpen, position, width, height },
  ref,
) => {
  const [css] = useStyletron();
  if (!position) {
    return null;
  }

  const displayTps = !tps ? "-" : tps;
  const displayTime = typeof time !== "number" ? "-" : formatUTCTimestamp(time, "DD.MM HH:mm");
  const tooltipContent = (
    <div className={css(styles.container)}>
      <LabelSmall color={COLORS.gray900}>{displayTime}</LabelSmall>
      <div className={css(styles.tpsContainer)}>
        <Marker $color={COLORS.blue400} />
        <LabelSmall color={COLORS.gray900}>MPS: {displayTps}</LabelSmall>
      </div>
    </div>
  );

  const tooltipOverrides: PopoverOverrides = {
    Body: {
      style: {
        zIndex: 1000,
        width: `${width}px`,
        height: `${height}px`,
        top: `${position.top}px`,
        left: `${position.left}px`,
      },
    },
  };

  return (
    <UIKitTooltip content={tooltipContent} isOpen={isOpen} overrides={tooltipOverrides}>
      <div className={css(styles.dummy)} ref={ref} />
    </UIKitTooltip>
  );
};

export const Tooltip = forwardRef(RenderFunc);
Tooltip.displayName = "Tooltip";
