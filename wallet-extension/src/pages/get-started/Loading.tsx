import { HeadingLarge } from "@nilfoundation/ui-kit";
import { useStore } from "effector-react";
import lottie from "lottie-web/build/player/lottie_light";
import { useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import animationData from "../../../public/animation/wallet-creation.json";
import { Box, Logo } from "../../features/components/shared";
import { $globalError } from "../../features/store/model/error.ts";
import { $balanceToken, fetchBalanceTokensFx } from "../../features/store/model/token.ts";
import { WalletRoutes } from "../../router";

export const Loading = () => {
  const navigate = useNavigate();
  const { t } = useTranslation("translation");
  const animationContainerRef = useRef<HTMLDivElement | null>(null);
  const isPending = useStore(fetchBalanceTokensFx.pending);
  const balanceCurrency = useStore($balanceToken);
  const globalError = useStore($globalError);

  useEffect(() => {
    // Initialize the Lottie animation
    const animationInstance = lottie.loadAnimation({
      container: animationContainerRef.current as Element,
      renderer: "svg",
      loop: true,
      autoplay: true,
      animationData,
    });

    // Cleanup on unmount
    return () => {
      animationInstance.destroy();
    };
  }, []);

  useEffect(() => {
    // Navigate to the error page if a global error is set
    if (globalError !== "" && globalError != null) {
      navigate(WalletRoutes.GET_STARTED.ERROR);
    }
  }, [globalError, navigate]);

  useEffect(() => {
    if (!isPending) {
      if (balanceCurrency == null) return;
      navigate(WalletRoutes.WALLET.BASE);
    }
  }, [isPending, balanceCurrency, navigate]);

  return (
    <Box
      $style={{
        height: "100vh",
        display: "flex",
        flexDirection: "column",
        justifyContent: "space-between",
        alignItems: "center",
        padding: "24px",
      }}
    >
      {/* Top: Logo */}
      <Box $align="center">
        <Logo size={40} />
      </Box>

      {/* Center: Lottie Animation and Heading */}
      <Box
        $align="center"
        $gap="16px"
        $padding="0"
        $style={{
          textAlign: "center",
          flexGrow: 1,
          display: "flex",
          flexDirection: "column",
          justifyContent: "center",
        }}
      >
        {/* Lottie Animation */}
        <div ref={animationContainerRef} style={{ width: 124, height: 124 }} />
        <HeadingLarge>{t("getStarted.loading.title")}</HeadingLarge>
      </Box>
    </Box>
  );
};
