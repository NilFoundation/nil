import { Button, COLORS, HeadingMedium, ParagraphXSmall } from "@nilfoundation/ui-kit";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import deleteIcon from "../../../public/icons/delete.svg";
import websiteIcon from "../../../public/icons/website.svg";
import { Box, Icon, ScreenHeader } from "../../features/components/shared";
import { WalletRoutes } from "../../router";

export const Connection = () => {
  const { t } = useTranslation("translation");
  const [endpoints, setEndpoints] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  // Fetch endpoints from Chrome storage
  const fetchEndpoints = async () => {
    chrome.storage.local.get("connectedWebsites", (result) => {
      const savedEndpoints = result.connectedWebsites || {};
      setEndpoints(Object.keys(savedEndpoints)); // Get the list of saved URLs
    });
  };

  // Fetch endpoints on page load
  useEffect(() => {
    fetchEndpoints();
  }, [fetchEndpoints]);

  // Handle Disconnect All
  const handleDisconnectAll = async () => {
    setIsLoading(true);
    await chrome.storage.local.remove("connectedWebsites"); // Clear storage
    await fetchEndpoints(); // Refresh list after clearing
    setIsLoading(false);
  };

  // Handle individual endpoint removal
  const handleRemoveEndpoint = async (url: string) => {
    try {
      chrome.storage.local.get("connectedWebsites", (result) => {
        const updatedEndpoints = { ...result.connectedWebsites };
        delete updatedEndpoints[url]; // Remove specific endpoint
        chrome.storage.local.set({ connectedWebsites: updatedEndpoints }, fetchEndpoints);
      });
    } catch (err) {
      console.error(err);
    }
  };

  return (
    <Box $justify="space-between" $padding="24px" $style={{ height: "100vh" }}>
      <Box>
        {/* Header */}
        <ScreenHeader
          route={WalletRoutes.WALLET.SETTINGS}
          title={t("wallet.connectionPage.title")}
        />

        {/* Endpoint List */}
        <Box
          $style={{
            maxHeight: "300px",
            overflowY: "auto",
            width: "100%",
            marginBottom: "16px",
            marginTop: "24px",
          }}
        >
          {endpoints.length > 0 &&
            endpoints.map((url) => (
              <Box
                key={url}
                $align="center"
                $gap="12px"
                $style={{ flexDirection: "row", marginBottom: "8px" }}
              >
                <Icon src={websiteIcon} alt="Website Icon" size={48} iconSize="100%" />
                <Box>
                  <HeadingMedium>{new URL(url).hostname}</HeadingMedium>
                  <ParagraphXSmall $style={{ color: COLORS.gray200 }}>{url}</ParagraphXSmall>
                </Box>
                <Icon
                  src={deleteIcon}
                  alt="Delete Icon"
                  size={32}
                  iconSize="100%"
                  $style={{ marginLeft: "auto", cursor: "pointer" }}
                  onClick={() => handleRemoveEndpoint(url)}
                />
              </Box>
            ))}
        </Box>
      </Box>

      {/* Disconnect All Button */}
      <Button
        onClick={handleDisconnectAll}
        isLoading={isLoading}
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
        Disconnect all
      </Button>
    </Box>
  );
};
