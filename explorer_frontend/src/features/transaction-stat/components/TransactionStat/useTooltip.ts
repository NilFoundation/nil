import type { MouseEventParams } from "lightweight-charts";

type UseToopltipReturn = {
  isOpen: boolean;
  position?: {
    left: number;
    top: number;
  };
};

const useTooltip = <T extends HTMLDivElement>(
  param?: MouseEventParams,
  container?: T | null,
  isMobile?: boolean,
  tooltipMargin = 10,
  toolTipWidth = 200,
): UseToopltipReturn => {
  if (isMobile) {
    return { isOpen: false };
  }

  if (!container || !param) {
    return { isOpen: false };
  }

  const isHidden =
    param.point === undefined ||
    !param.time ||
    param.point.x < 0 ||
    param.point.x > container.clientWidth ||
    param.point.y < 0 ||
    param.point.y > container.clientHeight;

  if (isHidden) {
    return { isOpen: false }; // early return to avoid unnecessary calculations
  }

  const y = param.point?.y ?? 0;
  const x = param.point?.x ?? 0;

  const left = x + (tooltipMargin + toolTipWidth) * 2;
  const top = y + container.clientHeight + tooltipMargin;

  return {
    isOpen: true,
    position: {
      left,
      top,
    },
  };
};

export { useTooltip };
