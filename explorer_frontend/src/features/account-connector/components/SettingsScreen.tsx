import { COLORS } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { useUnit } from "effector-react";
import { ActiveComponent } from "../ActiveComponent";
import { PrivateKeyIcon } from "../assets/PrivateKeyIcon";
import { ResetAccountIcon } from "../assets/ResetAccountIcon";
import { RpcUrlIcon } from "../assets/RpcUrlIcon";
import { $smartAccount, resetSmartAccount, setActiveComponent } from "../model";
import { BackLink } from "./BackLink";

const SettingRow = ({ icon, children, screen, isReset }) => {
  const [css, theme] = useStyletron();
  const alignItems = isReset ? "" : "center";
  const onClick = isReset
    ? () => {
        setActiveComponent(screen);
        resetSmartAccount();
      }
    : () => setActiveComponent(screen);

  return (
    <div
      className={css({
        display: "flex",
        flexDirection: "row",
        alignItems: alignItems,
        justifyContent: "flex-start",
        gap: "14px",
        padding: "10px 8px 10px 8px",
        borderRadius: "8px",
        ":hover": {
          backgroundColor: theme.colors.backgroundTertiary,
        },
        cursor: "pointer",
      })}
      onClick={onClick}
    >
      {icon}
      {children}
    </div>
  );
};

const SettingRowTextContent = ({ children }) => {
  const [css] = useStyletron();

  return (
    <div
      className={css({
        fontFamily: "Inter, sans-serif",
      })}
    >
      {children}
    </div>
  );
};

export const SettingsScreen = () => {
  const [css, theme] = useStyletron();

  const [smartAccount] = useUnit([$smartAccount]);

  const rpcUrlRowContent = <SettingRowTextContent>RPC URL</SettingRowTextContent>;
  const privateKeyRowContent = <SettingRowTextContent>View private key</SettingRowTextContent>;
  const resetAccountRowContent = (
    <SettingRowTextContent>
      <div
        className={css({
          display: "flex",
          flexDirection: "column",
          gap: "8px",
        })}
      >
        <div
          className={css({
            color: COLORS.red300,
          })}
        >
          Reset wallet
        </div>
        <div
          className={css({
            fontSize: "14px",
            color: COLORS.gray400,
          })}
        >
          Remove all data and generate a new smart account
        </div>
      </div>
    </SettingRowTextContent>
  );

  return (
    <div
      className={css({
        display: "flex",
        flexDirection: "column",
        gap: "12px",
      })}
    >
      <BackLink
        title="Settings"
        goBackCb={() => {
          setActiveComponent(ActiveComponent.Main);
        }}
      />
      <div
        className={css({
          width: "100%",
          display: "flex",
          flexDirection: "column",
          gap: "8px",
        })}
      >
        <SettingRow icon={<RpcUrlIcon />} screen={ActiveComponent.RpcUrlScreen} isReset={false}>
          {rpcUrlRowContent}
        </SettingRow>
        <SettingRow
          icon={<PrivateKeyIcon />}
          screen={ActiveComponent.PrivateKeyScreen}
          isReset={false}
        >
          {privateKeyRowContent}
        </SettingRow>
        <SettingRow
          icon={<ResetAccountIcon />}
          screen={ActiveComponent.AccountRegenerationScreen}
          isReset={true}
        >
          {resetAccountRowContent}
        </SettingRow>
      </div>
    </div>
  );
};
