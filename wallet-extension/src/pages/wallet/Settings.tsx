import { COLORS, ParagraphSmall } from "@nilfoundation/ui-kit";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import communityIcon from "../../../public/icons/community.svg";
import endpointIcon from "../../../public/icons/endpoint.svg";
import feedbackIcon from "../../../public/icons/feedback.svg";
import connectionIcon from "../../../public/icons/linked.svg";
import privateKeyIcon from "../../../public/icons/privatekey.svg";
import supportIcon from "../../../public/icons/support.svg";
import { Box, Icon, ScreenHeader } from "../../features/components/shared";
import { WalletRoutes } from "../../router";

export const Settings = () => {
  const { t } = useTranslation("translation");

  const links = [
    {
      titleKey: t("wallet.settingsPage.getSupport"),
      url: import.meta.env.VITE_APP_SUPPORT,
      icon: supportIcon,
    },
    {
      titleKey: t("wallet.settingsPage.feedback"),
      url: import.meta.env.VITE_APP_FEEDBACK,
      icon: feedbackIcon,
    },
    {
      titleKey: t("wallet.settingsPage.community"),
      url: import.meta.env.VITE_APP_COMMUNITY,
      icon: communityIcon,
    },
  ];

  const appVersion = import.meta.env.VITE_APP_VERSION || "1.0";
  const navigate = useNavigate();

  const handleEndpoint = () => {
    navigate(WalletRoutes.WALLET.ENDPOINT);
  };

  const handleConnection = () => {
    navigate(WalletRoutes.WALLET.CONNECTIONS);
  };

  const handlePrivateKey = () => {
    navigate(WalletRoutes.WALLET.PRIVATE_KEY);
  };

  return (
    <Box
      $align="stretch"
      $justify="space-between"
      $padding="24px"
      $style={{
        height: "100vh",
        boxSizing: "border-box",
      }}
    >
      {/* Header */}
      <ScreenHeader route={WalletRoutes.WALLET.BASE} title={t("wallet.settingsPage.title")} />

      {/* Endpoint Section */}
      <Box
        $align="center"
        $gap="8px"
        $padding="0"
        $style={{
          flexDirection: "row",
          cursor: "pointer",
          color: COLORS.gray50,
          ":hover": { color: COLORS.gray200 },
          marginTop: "48px",
          marginBottom: "24px",
        }}
        onClick={handleEndpoint}
      >
        <Icon
          src={endpointIcon}
          alt={"endpoint icon"}
          size={24}
          iconSize="100%"
          background="transparent"
        />
        <ParagraphSmall $style={{ color: "inherit" }}>
          {t("wallet.settingsPage.endpointSection")}
        </ParagraphSmall>
      </Box>

      {/* Private Key Section */}
      <Box
        $align="center"
        $gap="8px"
        $padding="0"
        $style={{
          flexDirection: "row",
          cursor: "pointer",
          color: COLORS.gray50,
          ":hover": { color: COLORS.gray200 },
          marginTop: "48px",
          marginBottom: "24px",
        }}
        onClick={handlePrivateKey}
      >
        <Icon
          src={privateKeyIcon}
          alt={"private key icon"}
          size={24}
          iconSize="100%"
          background="transparent"
        />
        <ParagraphSmall $style={{ color: "inherit" }}>
          {t("wallet.settingsPage.privateKeySection")}
        </ParagraphSmall>
      </Box>

      {/* Connections Section */}
      <Box
        $align="center"
        $gap="8px"
        $padding="0"
        $style={{
          flexDirection: "row",
          cursor: "pointer",
          color: COLORS.gray50,
          ":hover": { color: COLORS.gray200 },
          marginBottom: "24px",
        }}
        onClick={handleConnection}
      >
        <Icon
          src={connectionIcon}
          alt={"endpoint icon"}
          size={24}
          iconSize="100%"
          background="transparent"
        />
        <ParagraphSmall $style={{ color: "inherit" }}>
          {t("wallet.settingsPage.manageConnection")}
        </ParagraphSmall>
      </Box>

      {/* Settings Links */}
      <Box $gap="24px">
        {links.map((link) => (
          <a
            href={link.url}
            key={link.titleKey}
            target="_blank"
            rel="noopener noreferrer"
            style={{ textDecoration: "none" }}
          >
            <Box
              $align="center"
              $gap="8px"
              $padding="0"
              $style={{
                flexDirection: "row",
                cursor: "pointer",
                color: COLORS.gray50,
                ":hover": { color: COLORS.gray200 },
              }}
            >
              <Icon
                src={link.icon}
                alt={link.titleKey}
                size={24}
                iconSize="100%"
                background="transparent"
              />
              <ParagraphSmall $style={{ color: "inherit" }}>{link.titleKey}</ParagraphSmall>
            </Box>
          </a>
        ))}
      </Box>

      {/* Footer */}
      <ParagraphSmall
        $style={{
          textAlign: "center",
          color: COLORS.gray200,
          marginTop: "auto",
        }}
      >
        {t("wallet.settingsPage.footerVersion")} {appVersion}
      </ParagraphSmall>
    </Box>
  );
};
