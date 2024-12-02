import { useStyletron } from "styletron-react";
import type { StylesObject } from "../../shared";
import { COLORS, FormControl, Input, INPUT_KIND, INPUT_SIZE } from "@nilfoundation/ui-kit";
import { $endpoint, setEndpoint, topUpWalletBalanceFx } from "../model";
import { useUnit } from "effector-react";
import type { InputOverrides } from "baseui/input";
import type { FC } from "react";

type EndpointInputProps = {
  disabled?: boolean;
};

const styles: StylesObject = Object.freeze({
  container: {
    display: "flex",
    justifyContent: "center",
    flexDirection: "column",
    gap: "8px",
    alignItems: "center",
    marginLeft: "8px",
    width: "100%",
  },
});

const inputOverrides: InputOverrides = {
  Root: {
    style: () => ({
      background: COLORS.gray700,
      ":hover": {
        background: COLORS.gray600,
      },
    }),
  },
};

const EndpointInput: FC<EndpointInputProps> = ({ disabled }) => {
  const [endpoint] = useUnit([$endpoint, topUpWalletBalanceFx.pending]);
  const [css] = useStyletron();

  return (
    <div className={css(styles.container)}>
      <FormControl label="Endpoint">
        <Input
          kind={INPUT_KIND.secondary}
          size={INPUT_SIZE.small}
          overrides={inputOverrides}
          onChange={(e) => setEndpoint(e.target.value)}
          value={endpoint || ""}
          disabled={disabled}
        />
      </FormControl>
    </div>
  );
};

export { EndpointInput };
