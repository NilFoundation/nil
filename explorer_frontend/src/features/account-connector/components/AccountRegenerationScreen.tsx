import { COLORS, LabelLarge, LabelSmall } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { useUnit } from "effector-react";
import lottie from "lottie-web/build/player/lottie_light";
import { useEffect, useRef, useState } from "react";
import { ActiveComponent } from "../ActiveComponent";
import animationData from "../assets/wallet-creation.json";
import {
  $initializingSmartAccountError,
  $initializingSmartAccountState,
  createSmartAccountFx,
  setActiveComponent,
} from "../model";
import { BackLink } from "./BackLink";

export const AccountRegenerationScreen = () => {
  const [css, theme] = useStyletron();
  const [error, setError] = useState("");
  const [isDisabled, setIsDisabled] = useState(false);

  const animationContainerRef = useRef<HTMLDivElement | null>(null);

  const [
    initializingSmartAccountState,
    initializingSmartAccountError,
    isPendingSmartAccountCreation,
  ] = useUnit([
    $initializingSmartAccountState,
    $initializingSmartAccountError,
    createSmartAccountFx.pending,
  ]);

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

  useEffect(() => {
    if (initializingSmartAccountError) {
      setIsDisabled(false);
      setError(initializingSmartAccountError);
    }
  }, [initializingSmartAccountError]);

  return (
    <div
      className={css({
        display: "flex",
        flexDirection: "column",
        minHeight: "400px", // Or use 100vh to cover the entire viewport height
      })}
    >
      {/* Optional fixed header */}
      {!isPendingSmartAccountCreation && (
        <div>
          <BackLink
            title="Regenerate smart account"
            goBackCb={() => {
              setActiveComponent(ActiveComponent.SettingsScreen);
            }}
          />
        </div>
      )}

      {/* Main Content Container */}
      <div
        className={css({
          flex: 1,
          display: "flex",
          flexDirection: "column",
          justifyContent: "center",
          alignItems: "center",
          textAlign: "center",
          gap: "16px",
          padding: "0 16px",
        })}
      >
        {isPendingSmartAccountCreation ? (
          <>
            <div ref={animationContainerRef} style={{ width: 124, height: 124 }} />
            <LabelLarge style={{ color: COLORS.gray200 }}>
              {initializingSmartAccountState}
            </LabelLarge>
            <LabelSmall style={{ color: COLORS.red400 }}>{error}</LabelSmall>
          </>
        ) : (
          <>
            <LabelSmall style={{ color: COLORS.green400 }}>
              A new smart account has been created!
            </LabelSmall>
            <LabelSmall style={{ color: COLORS.red400 }}>{error}</LabelSmall>
          </>
        )}
      </div>
    </div>
  );
};
