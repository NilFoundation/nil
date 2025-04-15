import { BUTTON_KIND, Button, COLORS, Input, LabelMedium } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { useUnit } from "effector-react";
import { useState } from "react";
import { ActiveComponent } from "../ActiveComponent";
import CheckmarkIcon from "../assets/checkmark.svg";
import { $privateKey, $smartAccount, setActiveComponent } from "../model";
import { BackLink } from "./BackLink";

export const PrivateKeyScreen = () => {
  const [css, theme] = useStyletron();
  const [privateKey] = useUnit([$privateKey, $smartAccount]);
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    if (privateKey && typeof privateKey === "string" && privateKey.startsWith("0x")) {
      await navigator.clipboard.writeText(privateKey);

      setCopied(true);

      // Reset text after 2 seconds
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "flex-start",
        textAlign: "center",
        minHeight: "400px",
      }}
    >
      <BackLink
        title="View private key"
        goBackCb={() => {
          setActiveComponent(ActiveComponent.SettingsScreen);
        }}
      />

      <div
        className={css({
          display: "flex",
          width: "100%",
          gap: "12px",
          flexDirection: "column",
          alignItems: "start",
          marginTop: "12px",
        })}
      >
        <LabelMedium>Private key</LabelMedium>
        <div
          className={css({
            display: "flex",
            flexDirection: "row",
            gap: "12px",
            justifyContent: "space-around",
          })}
        >
          {/* Read-Only Input */}
          <Input
            placeholder="Private key"
            value={privateKey}
            readOnly
            overrides={{
              Root: {
                style: {
                  flex: 1,
                  height: "48px",
                  backgroundColor: theme.colors.rpcUrlBackgroundColor,
                  ":hover": {
                    backgroundColor: theme.colors.rpcUrlBackgroundHoverColor,
                  },
                  boxShadow: "none",
                },
              },
            }}
          />
          {/* Copy Button */}
          <Button
            kind={BUTTON_KIND.secondary}
            onClick={handleCopy}
            overrides={{
              Root: {
                style: {
                  width: "120px",
                  padding: "0 12px",
                  height: "48px",
                  backgroundColor: COLORS.gray50,
                  color: COLORS.gray800,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  gap: "8px",
                  ":hover": {
                    backgroundColor: COLORS.gray100,
                    color: COLORS.gray800,
                  },
                },
              },
            }}
          >
            {copied ? <img src={CheckmarkIcon} alt="Copied" width={20} height={20} /> : null}
            {copied ? "Copied!" : "Copy"}
          </Button>
        </div>
      </div>
    </div>
  );
};
