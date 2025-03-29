import { Button, COLORS, Input } from "@nilfoundation/ui-kit";
import { useStore } from "effector-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import CheckmarkIcon from "../../../public/icons/checkmark.svg";
import { Box, Icon, ScreenHeader } from "../../features/components/shared";
import { $endpoint } from "../../features/store/model/endpoint";
import { WalletRoutes } from "../../router";

export const Endpoint = () => {
  const { t } = useTranslation("translation");

  // Get the current endpoint from Effector
  const endpointValue = useStore($endpoint);

  // State for button loading and text
  const [showCheckmark, setShowCheckmark] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [buttonText, setButtonText] = useState(t("wallet.endpointPage.copyButton"));

  // Called when user clicks "Get Endpoint" (opens a new tab)
  const handleGetEndpoint = async () => {
    const endpointUrl = import.meta.env.VITE_GET_ENDPOINT_URL;
    if (endpointUrl) {
      await chrome.tabs.create({ url: endpointUrl });
    } else {
      console.error("Environment variable VITE_GET_ENDPOINT_URL is not set.");
    }
  };

  // Copy the endpoint to clipboard
  const handleCopyEndpoint = async () => {
    // Only copy if there's something in endpointValue
    if (endpointValue.trim()) {
      setIsLoading(true); // Set loading state to true
      try {
        await navigator.clipboard.writeText(endpointValue); // Copy to clipboard
        console.log("Endpoint copied:", endpointValue);

        // Update the button text to "Copied"
        setButtonText(t("wallet.endpointPage.copiedButton"));
        setShowCheckmark(true);

        // Reset button text and hide checkmark after 10 seconds
        setTimeout(() => {
          setButtonText(t("wallet.endpointPage.copyButton"));
          setShowCheckmark(false);
        }, 10000);
      } catch (error) {
        console.error("Error copying endpoint:", error);
      } finally {
        setIsLoading(false);
      }
    }
  };

  return (
    <Box
      $align="stretch"
      $justify="flex-start"
      $padding="24px"
      $style={{
        height: "100vh",
        boxSizing: "border-box",
      }}
    >
      {/* Header */}
      <ScreenHeader route={WalletRoutes.WALLET.SETTINGS} title={t("wallet.endpointPage.title")} />

      {/* Input + Copy Button */}
      <Box
        $align="center"
        $justify="space-between"
        $gap="5px"
        $style={{ paddingTop: "25px", flexDirection: "row", width: "100%" }}
      >
        <Input
          id="walletAddress"
          placeholder={t("wallet.endpointPage.inputPlaceholder")}
          value={endpointValue}
          readOnly
          overrides={{
            Root: {
              style: {
                flex: "1",
                height: "48px",
                // Make the text ellipsis if it's too long
                whiteSpace: "nowrap",
                overflow: "hidden",
                textOverflow: "ellipsis",
              },
            },
          }}
        />

        <Button
          onClick={handleCopyEndpoint}
          isLoading={isLoading}
          overrides={{
            Root: {
              style: {
                width: "120px",
                padding: "0 12px",
                height: "48px",
                backgroundColor: COLORS.gray50,
                color: COLORS.gray800,
                ":hover": {
                  backgroundColor: COLORS.gray100,
                },
              },
            },
          }}
        >
          {showCheckmark ? (
            <Box $align="center" $justify={"center"} $style={{ flexDirection: "row", gap: "8px" }}>
              <Icon
                src={CheckmarkIcon}
                size={16}
                alt="checkmark"
                // color={COLORS.gray200}
                iconSize="100%"
                background={"transparent"}
              />
              {buttonText}
            </Box>
          ) : (
            buttonText
          )}
        </Button>
      </Box>

      {/* Button Section */}
      <Box $style={{ marginTop: "auto", width: "100%" }}>
        <Button
          onClick={handleGetEndpoint}
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
          {t("wallet.endpointPage.getEndpointButton")}
        </Button>
      </Box>
    </Box>
  );
};
