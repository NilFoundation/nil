import { useStyletron } from "styletron-react";
import type { StylesObject } from "../../shared";
import { COLORS, FormControl, Input, INPUT_KIND, INPUT_SIZE } from "@nilfoundation/ui-kit";
import { $endpoint, setEndpoint } from "../models/model";
import { useUnit } from "effector-react";
import type { InputOverrides } from "baseui/input";

const styles: StylesObject = Object.freeze({
  container: {
    display: "flex",
    justifyContent: "center",
    flexDirection: "column",
    gap: "8px",
    alignItems: "center",
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

const EndpointInput = () => {
  const [endpoint] = useUnit([$endpoint]);
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
        />
      </FormControl>
    </div>
  );
};

export { EndpointInput };
