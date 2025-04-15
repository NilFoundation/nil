import { BUTTON_KIND, Button, COLORS, Input, LabelMedium } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import type { ButtonOverrides } from "baseui/button";
import { useUnit } from "effector-react";
import { useState } from "react";
import { getRuntimeConfigOrThrow } from "../../runtime-config/getRuntimeConfigOrThrow";
import { ActiveComponent } from "../ActiveComponent";
import CheckmarkIcon from "../assets/checkmark.svg";
import { $rpcUrl, setActiveComponent } from "../model";
import { BackLink } from "./BackLink";

const btnOverrides: ButtonOverrides = {
  Root: {
    style: ({ $disabled }) => ({
      backgroundColor: $disabled ? `${COLORS.gray400}!important` : "",
      width: "100%",
    }),
  },
};

export const RpcUrlScreen = () => {
  const [css, theme] = useStyletron();

  const [isDisabled] = useState(false);
  const { RPC_TELEGRAM_BOT } = getRuntimeConfigOrThrow();

  const [rpcUrl] = useUnit([$rpcUrl]);

  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    if (rpcUrl && typeof rpcUrl === "string") {
      await navigator.clipboard.writeText(rpcUrl);

      setCopied(true);

      // Reset text after 2 seconds
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleGetRpcUrl = () => {
    if (RPC_TELEGRAM_BOT) {
      window.open(RPC_TELEGRAM_BOT, "_blank");
    } else {
      console.error("RPC_TELEGRAM_BOT runtime variable is not set.");
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
        title="View RPC URL"
        goBackCb={() => {
          setActiveComponent(ActiveComponent.SettingsScreen);
        }}
      />
      {rpcUrl !== null && (
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
          <LabelMedium>RPC URL</LabelMedium>
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
              placeholder="Enter your RPC URL"
              value={rpcUrl}
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
      )}

      <div
        className={css({
          flexGrow: 1,
        })}
      />
      <div
        style={{
          width: "100%",
          maxWidth: "400px",
          display: "flex",
          flexDirection: "column",
          gap: "8px",
        }}
      >
        <Button
          onClick={handleGetRpcUrl}
          kind={BUTTON_KIND.secondary}
          disabled={isDisabled}
          style={{
            width: "100%",
            height: "48px",
            backgroundColor: COLORS.gray700,
            color: COLORS.gray200,
          }}
          overrides={btnOverrides}
        >
          Get RPC URL
        </Button>
      </div>
    </div>
  );
};
