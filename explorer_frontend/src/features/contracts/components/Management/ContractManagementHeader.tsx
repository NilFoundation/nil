import {
  ArrowUpIcon,
  BUTTON_KIND,
  BUTTON_SIZE,
  Button,
  COLORS,
  LabelMedium,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "styletron-react";
import { closeApp } from "../../model";
import { RemoveAppButton } from "../RemoveAppButton";
import type { Hex } from "@nilfoundation/niljs";
import type { FC } from "react";
import { OverflowEllipsis } from "../../../shared";

type ContractManagementHeaderProps = {
  address: Hex;
  bytecode: Hex;
  name: string;
  loading?: boolean;
};

export const ContractManagementHeader: FC<ContractManagementHeaderProps> = ({
  address,
  bytecode,
  name,
  loading,
}) => {
  const [css] = useStyletron();

  return (
    <div
      className={css({
        display: "flex",
        gap: "12px",
        alignItems: "center",
        position: "sticky",
        top: "-1px",
        backgroundColor: COLORS.gray900,
        paddingTop: "16px",
        paddingBottom: "16px",
        zIndex: 2,
      })}
    >
      <Button
        overrides={{
          Root: {
            style: {
              paddingLeft: 0,
              paddingRight: 0,
              width: "32px",
              height: "32px",
              flexShrink: 0,
            },
          },
        }}
        disabled={loading}
        kind={BUTTON_KIND.secondary}
        size={BUTTON_SIZE.compact}
        onClick={() => closeApp()}
      >
        <ArrowUpIcon
          size={12}
          className={css({
            transform: "rotate(-90deg)",
          })}
        />
      </Button>
      <LabelMedium color={COLORS.gray50}>{name}</LabelMedium>
      <LabelMedium
        className={css({
          width: "max(calc(100% - 250px), 100px)",
          marginRight: "auto",
        })}
      >
        <OverflowEllipsis charsFromTheEnd={5}>{address}</OverflowEllipsis>
      </LabelMedium>
      <RemoveAppButton
        disabled={loading}
        address={address}
        bytecode={bytecode}
        kind={BUTTON_KIND.secondary}
      />
    </div>
  );
};
