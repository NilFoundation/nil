import type { FC } from "react";
import {
  type PopoverProps,
  StatefulPopover as StatefulPopoverBase,
  type StatefulPopoverProps,
  Popover as PopoverBase,
} from "baseui/popover";

const StatefulPopover: FC<StatefulPopoverProps> = ({ ...props }) => {
  return <StatefulPopoverBase {...props} dismissOnEsc />;
};

const Popover: FC<PopoverProps> = ({ ...props }) => {
  return <PopoverBase {...props} />;
};

export { StatefulPopover, Popover };
