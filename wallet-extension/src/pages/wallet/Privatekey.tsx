import { Button, COLORS, Input } from "@nilfoundation/ui-kit";
import { useStore } from "effector-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import CheckmarkIcon from "../../../public/icons/checkmark.svg";
import { Box, Icon, ScreenHeader } from "../../features/components/shared";
import { $privateKey } from "../../features/store/model/privateKey";
import { WalletRoutes } from "../../router";

export const PrivateKey = () => {
  const { t } = useTranslation("translation");

  // Get the current private key from Effector
  const privateKeyValue = useStore($privateKey);

  // State for button loading and text
  const [showCheckmark, setShowCheckmark] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [buttonText, setButtonText] = useState(t("wallet.privateKeyPage.copyButton"));

  // Copy the private key to clipboard
  const handleCopyPrivateKey = async () => {
    if (!privateKeyValue) {
      console.error("No private key found");
      return;
    }

    // Only copy if there's something in privateKeyValue
    if (privateKeyValue.trim()) {
      setIsLoading(true); // Set loading state to true
      try {
        await navigator.clipboard.writeText(privateKeyValue); // Copy to clipboard
        console.log("Private key copied");

        // Update the button text to "Copied"
        setButtonText(t("wallet.privateKeyPage.copiedButton"));
        setShowCheckmark(true);

        // Reset button text and hide checkmark after 10 seconds
        setTimeout(() => {
          setButtonText(t("wallet.privateKeyPage.copyButton"));
          setShowCheckmark(false);
        }, 10000);
      } catch (error) {
        console.error("Error copying private key:", error);
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
      <ScreenHeader route={WalletRoutes.WALLET.SETTINGS} title={t("wallet.privateKeyPage.title")} />

      {/* Input + Copy Button */}
      <Box
        $align="center"
        $justify="space-between"
        $gap="5px"
        $style={{ paddingTop: "25px", flexDirection: "row", width: "100%" }}
      >
        <Input
          id="walletAddress"
          placeholder={t("wallet.privateKeyPage.inputPlaceholder")}
          value={privateKeyValue || ""}
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
          onClick={handleCopyPrivateKey}
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
    </Box>
  );
};
