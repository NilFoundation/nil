import type { Hex } from "@nilfoundation/niljs";
import { Button, COLORS, FormControl, Input, SPACE, LabelSmall } from "@nilfoundation/ui-kit";
import { INPUT_KIND, INPUT_SIZE } from "@nilfoundation/ui-kit";
import type { InputOverrides } from "baseui/input";
import { useUnit } from "effector-react";
import { useStyletron } from "styletron-react";
import { $smartAccount } from "../../../account-connector/model";
import {
  assignSmartContract,
  assignSmartContractFx,
  setAssignedSmartContractAddress,
  $state,
  $assignedSmartContractAddress,
} from "../../models/base";
import { useState, useEffect } from "react";

export const AssignTab = () => {
<<<<<<< HEAD
  const [smartAccount, pending, existingContractsList, assignedAddress] = useUnit([
=======
  const [smartAccount, pending, state, assignedAddress] = useUnit([
>>>>>>> 87baef7 (explorer bugs resolved)
    $smartAccount,
    assignSmartContractFx.pending,
    $state,
    $assignedSmartContractAddress,
  ]);

  const [css] = useStyletron();
<<<<<<< HEAD
  const [error, setError] = useState<string | null>(null); 
=======
  const [error, setError] = useState<string | null>(null); // Track error state

  // Validate the address input
>>>>>>> 87baef7 (explorer bugs resolved)
  const validateAddress = (address: string) => {
    if (!address || address === "0x") {
      setError(null);
      return;
    }

<<<<<<< HEAD
    const existingAddresses = Object.values(existingContractsList).flat();
=======
    // Flatten current state addresses and check if the address exists
    const existingAddresses = Object.values(state).flat();
>>>>>>> 87baef7 (explorer bugs resolved)
    if (existingAddresses.includes(address)) {
      setError(`Contract with address ${address} already exists.`);
    } else {
      setError(null);
    }
  };

  useEffect(() => {
<<<<<<< HEAD
=======
    // Validate whenever the assigned address changes
>>>>>>> 87baef7 (explorer bugs resolved)
    validateAddress(assignedAddress);
  }, [assignedAddress, state]);

  useEffect(() => {
<<<<<<< HEAD
    setAssignedSmartContractAddress("0x" as Hex); 
    setError(null);
  }, []); 
=======
    // Clear the assigned address and reset the error when the tab is reopened
    setAssignedSmartContractAddress("0x" as Hex); // Reset to "0x" or empty string
    setError(null);
  }, []); // Runs only once when the component mounts
>>>>>>> 87baef7 (explorer bugs resolved)

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
              const value = e.target.value as Hex;
              setAssignedSmartContractAddress(value);
            }}
            value={assignedAddress && assignedAddress !== "0x" ? assignedAddress : ""}
          />
        </FormControl>
        {error && (
          <LabelSmall
            className={css({
              color: COLORS.red500,
              marginTop: SPACE[4],
            })}
          >
            {error}
          </LabelSmall>
        )}
      </div>
      <div>
        <Button
          onClick={() => {
            if (!error) assignSmartContract();
          }}
          isLoading={pending}
          disabled={pending || !smartAccount || !!error}
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