import type { FC } from "react";
import { StatefulPopover, type StatefulPopoverProps } from "baseui/popover";

const Popover: FC<StatefulPopoverProps> = ({ ...props }) => {
  return <StatefulPopover {...props} dismissOnEsc autoFocus placement="bottomRight" />;
};

export { Popover };
