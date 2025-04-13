import { FormControl, HeadingLarge, Input, type InputProps } from "@nilfoundation/ui-kit";
import { BUTTON_KIND, Button, COLORS, LabelLarge, LabelSmall } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import type { ButtonOverrides } from "baseui/button";
import { useUnit } from "effector-react";
import lottie from "lottie-web/build/player/lottie_light";
import { useEffect, useRef, useState } from "react";
import { getRuntimeConfigOrThrow } from "../../runtime-config";
import { ActiveComponent } from "../ActiveComponent.ts";
import asteriskIcon from "../assets/asterisk.svg";
import animationData from "../assets/wallet-creation.json";
import {
  $balance,
  $balanceToken,
  $initializingSmartAccountError,
  $initializingSmartAccountState,
  $rpcUrl,
  $smartAccount,
  createSmartAccountFx,
  initilizeSmartAccount,
  setActiveComponent,
} from "../model";

const btnOverrides: ButtonOverrides = {
  Root: {
    style: ({ $disabled }) => ({
      backgroundColor: $disabled ? `${COLORS.gray400}!important` : "",
      width: "100%",
    }),
  },
};

export const RpcUrlScreen = () => {
  const [css] = useStyletron();
  const [error, setError] = useState("");
  const [isDisabled, setIsDisabled] = useState(false);
  const { RPC_TELEGRAM_BOT } = getRuntimeConfigOrThrow();

  // Get initialization states
  const [
    smartAccount,
    balance,
    balanceToken,
    initializingSmartAccountState,
    initializingSmartAccountError,
    isPendingSmartAccountCreation,
    rpcUrl,
  ] = useUnit([
    $smartAccount,
    $balance,
    $balanceToken,
    $initializingSmartAccountState,
    $initializingSmartAccountError,
    createSmartAccountFx.pending,
    $rpcUrl,
  ]);
  const [inputValue, setInputValue] = useState(rpcUrl);
  const animationContainerRef = useRef<HTMLDivElement | null>(null);

  // biome-ignore lint/correctness/useExhaustiveDependencies: <explanation>
  useEffect(() => {
    if (animationContainerRef.current) {
      const animationInstance = lottie.loadAnimation({
        container: animationContainerRef.current as Element,
        renderer: "svg",
        loop: true,
        autoplay: true,
        animationData,
      });

      return () => animationInstance.destroy();
    }
  }, [isPendingSmartAccountCreation]);

  const handleConnect = () => {
    const trimmedInputValue = inputValue.trim();
    setIsDisabled(true);
    initilizeSmartAccount(trimmedInputValue);
  };

  const handleGetRpcUrl = () => {
    if (RPC_TELEGRAM_BOT) {
      window.open(RPC_TELEGRAM_BOT, "_blank");
    } else {
      console.error("RPC_TELEGRAM_BOT runtime variable is not set.");
    }
  };

  const handleInputChange: InputProps["onChange"] = (e) => {
    setError("");
    setInputValue(e.target.value);
  };

  useEffect(() => {
    if (smartAccount !== null && balance !== null && balanceToken !== null) {
      setActiveComponent(ActiveComponent.Main);
    }
  }, [smartAccount, balance, balanceToken]);

  useEffect(() => {
    if (initializingSmartAccountError) {
      setIsDisabled(false);
      setError(initializingSmartAccountError);
    }
  }, [initializingSmartAccountError]);

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "space-between",
        textAlign: "center",
        minHeight: "400px",
      }}
    >
      {!isPendingSmartAccountCreation ? (
        <div style={{ width: "100%" }}>
          <img src={asteriskIcon} alt="Asterisk Icon" width={124} height={124} />
          <HeadingLarge>Enter the RPC URL to connect the wallet</HeadingLarge>
        </div>
      ) : (
        <div
          className={css({
            width: "100%",
            "@media (max-width: 419px)": {
              width: "calc(100vw - 48px)",
            },
          })}
        >
          <div
            ref={animationContainerRef}
            style={{
              width: 124,
              height: 124,
              justifyContent: "center",
              alignItems: "center",
              margin: "0 auto",
            }}
          />
          <LabelLarge style={{ color: COLORS.gray200, marginTop: "16px" }}>
            {initializingSmartAccountState}
          </LabelLarge>
        </div>
      )}

      <FormControl error={error !== ""}>
        <Input
          placeholder="Enter your RPC URL"
          value={inputValue}
          onChange={handleInputChange}
          disabled={isDisabled}
          overrides={{
            Root: {
              style: {
                width: "100%",
                height: "48px",
                marginTop: "12px",
                background: COLORS.gray700,
                ":hover": {
                  background: COLORS.gray600,
                },
              },
            },
          }}
        />
      </FormControl>

      <LabelSmall style={{ color: COLORS.red400, marginBottom: "8px" }}>{error}</LabelSmall>

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
          onClick={handleConnect}
          kind={BUTTON_KIND.primary}
          disabled={inputValue.trim() === ""}
          isLoading={isPendingSmartAccountCreation}
          style={{ width: "100%", height: "48px" }}
          overrides={btnOverrides}
        >
          Connect
        </Button>
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
