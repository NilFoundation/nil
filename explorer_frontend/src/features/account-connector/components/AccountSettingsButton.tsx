import { BUTTON_KIND, ButtonIcon } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { ActiveComponent } from "../ActiveComponent";
import { SettingsIcon } from "../assets/SettingsIcon";
import { setActiveComponent } from "../model";

export const AccountSettingsButton = () => {
  const [css, theme] = useStyletron();

  return (
    <ButtonIcon
      className={css({
        width: "32px",
        height: "32px",
        flexShrink: 0,
        backgroundColor: `${theme.colors.accountSettingsButtonBackgroundColor} !important`,
        ":hover": {
          backgroundColor: `${theme.colors.accountSettingsButtonBackgroundHoverColor} !important`,
        },
      })}
      icon={<SettingsIcon />}
      kind={BUTTON_KIND.secondary}
      onClick={() => setActiveComponent(ActiveComponent.SettingsScreen)}
    />
  );
};
