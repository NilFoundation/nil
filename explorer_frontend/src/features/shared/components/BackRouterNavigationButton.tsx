import { ArrowUpIcon, BUTTON_KIND, BUTTON_SIZE, ButtonIcon, COLORS } from "@nilfoundation/ui-kit";
import { mergeOverrides } from "baseui";
import type { ButtonOverrides } from "baseui/button";
import { useUnit } from "effector-react";
import type { FC } from "react";
import { router } from "../../routing";
import { isTutorialPage } from "../../code/model";

type BackButtonProps = {
  overrides?: ButtonOverrides;
  disabled?: boolean;
};

export const BackRouterNavigationButton: FC<BackButtonProps> = ({ overrides, disabled }) => {
  const [history, isTutorial] = useUnit([router.$history, isTutorialPage]);
  const historyEmpty = window.history.length < 2;
  const mergedOverrides = mergeOverrides(
    {
      Root: {
        style: {
          transform: "rotate(-90deg)",
          width: "48px",
          height: "48px",
          ...(isTutorial && {
            backgroundColor: COLORS.blue800,
            ':hover': {
              backgroundColor: COLORS.blue700,
            }
          })
        },
      },
    },
    overrides,
  );

  return (
    <ButtonIcon
      icon={<ArrowUpIcon $size={"16px"} />}
      kind={BUTTON_KIND.secondary}
      size={BUTTON_SIZE.large}
      onClick={() => history.back()}
      overrides={mergedOverrides}
      disabled={historyEmpty || disabled}
    />
  );
};
