import { Button, COLORS, FormControl, Input, SPACE } from "@nilfoundation/ui-kit";
import { INPUT_KIND, INPUT_SIZE } from "@nilfoundation/ui-kit";
import { useUnit } from "effector-react";
import {
  assignSmartContract,
  assignSmartContractFx,
  setAssignedSmartContractAddress,
} from "../../models/base";
import { useStyletron } from "styletron-react";
import { $wallet } from "../../../account-connector/model";
import type { InputOverrides } from "baseui/input";

import { useState } from "react";

export const AssignTab = () => {
  const [wallet, pending] = useUnit([$wallet, assignSmartContractFx.pending]);
  const [inputValue, setInputValue] = useState<string>("");
  const [css] = useStyletron();

  return (
    <>
      <div
        className={css({
          flexGrow: 0,
          paddingBottom: SPACE[16],
        })}
      >
        <FormControl label="Address">
          <Input
            kind={INPUT_KIND.secondary}
            size={INPUT_SIZE.small}
            overrides={inputOverrides}
            onChange={(e) => {
              setInputValue(e.target.value);
            }}
            value={inputValue}
            placeholder="0x000..."
          />
        </FormControl>
      </div>
      <div>
        <Button
          onClick={() => {
            if (inputValue) {
              setAssignedSmartContractAddress(inputValue);
              assignSmartContract();
            } else {
              console.error("Address is undefined");
            }
          }}
          isLoading={pending}
          disabled={pending || !wallet}
        >
          Assign
        </Button>
      </div>
    </>
  );
};

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
