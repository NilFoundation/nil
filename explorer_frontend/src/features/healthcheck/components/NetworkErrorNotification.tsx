import { COLORS, Notification, WarningIcon } from "@nilfoundation/ui-kit";
import type { FC } from "react";
import { useMobile } from "../../shared/hooks/useMobile";

export const NetworkErrorNotification: FC = () => {
  const [isMobile] = useMobile();
  return (
    <Notification
      overrides={{
        Body: {
          style: {
            position: "fixed",
            bottom: "20px",
            right: "16px",
            zIndex: 9999,
            background: COLORS.red800,
            width: isMobile ? "300px" : "450px",
          },
        },
      }}
      icon={<WarningIcon />}
      closeable={true}
    >
      Something is wrong with the network or you have an incorrect RPC url.
    </Notification>
  );
};
