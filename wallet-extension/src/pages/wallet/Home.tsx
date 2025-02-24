import type { Hex } from "@nilfoundation/niljs";
import {
  COLORS,
  NOTIFICATION_KIND,
  Notification,
  ParagraphSmall,
  ParagraphXSmall,
} from "@nilfoundation/ui-kit";
import { StatefulTooltip } from "baseui/tooltip";
import { useStore } from "effector-react";
import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import linkIcon from "../../../public/icons/link.svg";
import { ActivityType } from "../../background/storage";
import { Box, Icon } from "../../features/components/shared";
import { Header, QuickActions, ResourceTabs } from "../../features/components/wallet";
import { $latestActivity, clearLatestActivity } from "../../features/store/model/activities.ts";
import { $globalError } from "../../features/store/model/error.ts";
import { refetchBalancesEvent } from "../../features/store/model/token.ts";
import { formatAddress } from "../../features/utils";
import { WalletRoutes } from "../../router";

export const Home = () => {
  const navigate = useNavigate();
  const globalError = useStore($globalError);
  const latestActivity = useStore($latestActivity);

  useEffect(() => {
    // Navigate to the error page if a global error is set
    if (globalError !== "" && globalError !== null) {
      navigate(WalletRoutes.WALLET.ERROR);
    }
  }, [globalError, navigate]);

  useEffect(() => {
    // Trigger an immediate refetch when the Home page loads
    console.log("Home Page Loaded: Triggering initial refetch");
    refetchBalancesEvent();

    // Set up periodic refetching every 30 seconds
    const intervalId = setInterval(() => {
      console.log("Periodic refetch triggered");
      refetchBalancesEvent();
    }, 30000);
    // Cleanup the interval when the component unmounts
    return () => {
      console.log("Leaving Home Page: Clearing refetch interval");
      clearInterval(intervalId);
    };
  }, []);

  useEffect(() => {
    if (latestActivity) {
      console.log("New activity detected:", latestActivity);

      // Auto-hide the notification after 15 seconds
      const timeoutId = setTimeout(() => {
        clearLatestActivity();
      }, 15000);

      return () => clearTimeout(timeoutId);
    }
  }, [latestActivity]);

  const handleNavigate = () => {
    const endpointUrl = import.meta.env.VITE_NIL_EXPLORER;
    if (endpointUrl && latestActivity?.txHash) {
      const urlWithTx = `${endpointUrl}tx/${latestActivity.txHash}`;
      chrome.tabs.create({ url: urlWithTx });
    } else {
      console.error("Environment variable VITE_NIL_EXPLORER or transaction hash is not set.");
    }
  };

  return (
    <Box $padding="24px" $style={{ height: "100vh", boxSizing: "border-box" }} $gap={"32px"}>
      {/* Header */}
      <Header />

      {/* Quick Actions */}
      <QuickActions />

      {/* Tabs */}
      <ResourceTabs />

      {/* Notification for latest activity */}
      {latestActivity && (
        <Box
          style={{
            position: "fixed",
            bottom: "15px",
            left: 0,
            right: 0,
            zIndex: 1000,
            boxSizing: "border-box",
          }}
        >
          <Notification
            closeable={true}
            kind={NOTIFICATION_KIND.warning}
            hideIcon={true}
            icon={
              <StatefulTooltip
                content={() => "Open in Explorer"}
                showArrow={true}
                placement="top"
                overrides={{
                  Body: {
                    style: {
                      zIndex: 3000,
                    },
                  },
                  Inner: {
                    style: {
                      backgroundColor: COLORS.gray50,
                    },
                  },
                }}
              >
                <div
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    justifyContent: "center",
                    padding: "4px",
                  }}
                >
                  <Icon
                    src={linkIcon}
                    alt="Link"
                    size={24}
                    iconSize="100%"
                    background="transparent"
                    onClick={handleNavigate}
                    pointer={true}
                  />
                </div>
              </StatefulTooltip>
            }
            overrides={{
              Body: {
                style: {
                  backgroundColor: COLORS.gray700,
                  width: "100%",
                  maxWidth: "calc(100% - 32px)",
                  padding: "12px 16px",
                  borderRadius: "8px",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  boxSizing: "border-box",
                },
              },
            }}
            //autoHideDuration={5000}
          >
            {/* Left Content - Activity Details */}
            <Box $align="flex-start" onClick={handleNavigate}>
              <ParagraphSmall
                $style={{
                  color: COLORS.gray50,
                  cursor: "pointer",
                  transition: "color 0.2s ease-in-out",
                  ":hover": { color: COLORS.gray100 },
                }}
              >
                {latestActivity.activityType === ActivityType.SEND ? "Sent" : "Topped Up"}{" "}
                {latestActivity?.amount} {latestActivity?.token}
              </ParagraphSmall>

              <ParagraphXSmall
                $style={{
                  color: COLORS.gray200,
                  cursor: "pointer",
                  transition: "color 0.2s ease-in-out",
                  ":hover": { color: COLORS.gray300 },
                }}
              >
                {formatAddress(latestActivity?.txHash as Hex)}
              </ParagraphXSmall>
            </Box>
          </Notification>
        </Box>
      )}
    </Box>
  );
};
