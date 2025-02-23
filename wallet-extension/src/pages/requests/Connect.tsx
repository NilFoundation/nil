import {
  Button,
  COLORS,
  HeadingMedium,
  Notification,
  ParagraphSmall,
  ParagraphXSmall,
} from "@nilfoundation/ui-kit";
import { useStore } from "effector-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import walletIcon from "../../../public/icons/wallet.svg";
import websiteIcon from "../../../public/icons/website.svg";
import { Box, Icon } from "../../features/components/shared";
import { $smartAccount } from "../../features/store/model/smartAccount.ts";
import { formatAddress } from "../../features/utils";

export const Connect = () => {
  const { t } = useTranslation("translation");
  const [requestId, setRequestId] = useState<string>("");
  const [origin, setOrigin] = useState<string>("");
  const smartAccount = useStore($smartAccount);

  // Extract requestId and origin from the URL
  useEffect(() => {
    const fetchTransactionData = () => {
      const hash = window.location.hash; // e.g., "#/connect?requestId=123&origin=https%3A%2F%2Fexample.com"
      const queryString = hash.split("?")[1]; // Extract "requestId=123&origin=https%3A%2F%2Fexample.com"
      const urlParams = new URLSearchParams(queryString);

      const requestIdParam = urlParams.get("requestId");
      const originParam = urlParams.get("origin");

      if (requestIdParam) setRequestId(requestIdParam);
      if (originParam) setOrigin(decodeURIComponent(originParam));
    };

    fetchTransactionData();
  }, []);

  // Handle cancel action
  const handleCancel = () => {
    const connectPort = chrome.runtime.connect({ name: "connect-request" });
    connectPort.postMessage({ requestId, origin });

    // Delay window close to ensure message is sent
    setTimeout(() => {
      window.close();
    }, 100);
  };

  // Handle connect action
  const handleConnect = () => {
    const connectPort = chrome.runtime.connect({ name: "connect-request" });
    connectPort.postMessage({ requestId, origin, smartAccountAddress: smartAccount.address });

    // Delay window close to ensure message is sent
    setTimeout(() => {
      window.close();
    }, 100);
  };

  return (
    <Box $$justify="space-between" $padding="24px" $style={{ height: "100vh" }}>
      {/* Top: Title */}
      <HeadingMedium $style={{ color: COLORS.gray50, marginBottom: "24px" }}>
        {t("requests.connect.title")}
      </HeadingMedium>

      {/* Website Section */}
      <Box
        $align="flex-start"
        $direction="column"
        $gap="8px"
        $style={{ width: "100%", marginBottom: "12px" }}
      >
        <ParagraphSmall $style={{ color: COLORS.gray200 }}>
          {t("requests.connect.requestedLabel")}
        </ParagraphSmall>
        <Box $align="center" $gap="12px" $style={{ flexDirection: "row" }}>
          <Icon src={websiteIcon} alt="Website Icon" size={64} iconSize="100%" />
          <Box>
            <HeadingMedium>Name</HeadingMedium>
            <ParagraphXSmall $style={{ color: COLORS.gray200 }}>{origin}</ParagraphXSmall>
          </Box>
        </Box>
      </Box>

      {/* Wallet Connection Section */}
      <Box $align="flex-start" $direction="column" $gap="8px" $style={{ width: "100%" }}>
        <ParagraphSmall $style={{ color: COLORS.gray200 }}>
          {t("requests.connect.walletLabel")}
        </ParagraphSmall>
        <Box $align="center" $gap="12px" $style={{ flexDirection: "row" }}>
          <Icon src={walletIcon} alt="Wallet Icon" size={64} iconSize="100%" />
          <Box>
            <HeadingMedium>{t("requests.connect.walletName")}</HeadingMedium>
            <ParagraphXSmall $style={{ color: COLORS.gray200 }}>
              {formatAddress(smartAccount.address)}
            </ParagraphXSmall>
          </Box>
        </Box>
      </Box>

      {/* Empty space to separate sections */}
      <Box $style={{ flexGrow: 1 }} />

      {/* Notification */}
      <Notification
        closeable={false}
        kind="info"
        overrides={{
          Body: {
            style: {
              backgroundColor: COLORS.gray800,
              marginLeft: 0,
              marginRight: 0,
              width: "100%",
            },
          },
        }}
      >
        <ParagraphSmall>{t("requests.connect.notification")}</ParagraphSmall>
      </Notification>

      {/* Buttons */}
      <Box $align="center" $gap="8px" $style={{ width: "100%" }}>
        <Button
          onClick={handleConnect}
          overrides={{
            Root: {
              style: {
                width: "100%",
                height: "48px",
              },
            },
          }}
        >
          {t("requests.connect.connectButton")}
        </Button>

        <Button
          onClick={handleCancel}
          overrides={{
            Root: {
              style: {
                width: "100%",
                height: "48px",
                backgroundColor: COLORS.gray800,
                color: COLORS.gray200,
                ":hover": {
                  backgroundColor: COLORS.gray700,
                },
              },
            },
          }}
        >
          {t("requests.connect.cancelButton")}
        </Button>
      </Box>
    </Box>
  );
};
