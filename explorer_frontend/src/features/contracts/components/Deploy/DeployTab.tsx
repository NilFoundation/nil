import {
  Button,
  COLORS,
  Checkbox,
  FormControl,
  HeadingMedium,
  Input,
  ParagraphSmall,
  SPACE,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { useUnit } from "effector-react";
import { $smartAccount } from "../../../account-connector/model";
import { $constructor } from "../../init";
import {
  $deploySmartContractError,
  $deploymentArgs,
  $shardId,
  $shardIdIsValid,
  deploySmartContract,
  deploySmartContractFx,
  setDeploymentArg,
  setShardId,
} from "../../models/base";
import { ShardIdInput } from "./ShardIdInput";

export const DeployTab = () => {
  const [smartAccount, args, constuctorAbi, pending, shardId, shardIdIsValid, deployError] =
    useUnit([
      $smartAccount,
      $deploymentArgs,
      $constructor,
      deploySmartContractFx.pending,
      $shardId,
      $shardIdIsValid,
      $deploySmartContractError,
    ]);
  const [css, theme] = useStyletron();

  return (
    <>
      <div
        className={css({
          flexGrow: 0,
          paddingBottom: SPACE[16],
        })}
      >
        <FormControl
          label="SmartAccount"
          caption="From this smart account contract will be recorded to network"
        >
          <Input
            overrides={{
              Root: {
                style: {
                  marginBottom: SPACE[8],
                  backgroundColor: theme.colors.backgroundSecondary,
                  ":hover": {
                    backgroundColor: theme.colors.backgroundTertiary,
                  },
                },
              },
            }}
            name="SmartAccount"
            value={smartAccount?.address ?? ""}
            disabled
            readOnly
          />
        </FormControl>
      </div>
      <div>
        <ShardIdInput shardId={shardId} setShardId={setShardId} disabled={pending} />
        {constuctorAbi?.inputs.length ? (
          <div
            className={css({
              paddingTop: "16px",
              borderTop: `1px solid ${COLORS.gray800}`,
              borderBottom: `1px solid ${COLORS.gray800}`,
              marginBottom: "24px",
            })}
          >
            <HeadingMedium
              className={css({
                marginBottom: SPACE[8],
              })}
            >
              Deployment arguments
            </HeadingMedium>
            {constuctorAbi.inputs.map((input) => {
              if (typeof input.name !== "string") {
                return null;
              }
              const name = input.name;
              return (
                <FormControl label={name} caption={input.type} key={name}>
                  {input.type === "bool" ? (
                    <Checkbox
                      overrides={{
                        Root: {
                          style: {
                            marginBottom: SPACE[8],
                            backgroundColor: theme.colors.backgroundSecondary,
                            ":hover": {
                              backgroundColor: theme.colors.backgroundTertiary,
                            },
                          },
                        },
                      }}
                      key={name}
                      checked={typeof args[name] === "boolean" ? !!args[name] : false}
                      onChange={(e) => {
                        setDeploymentArg({ key: name, value: e.target.checked });
                      }}
                    />
                  ) : (
                    <Input
                      key={name}
                      overrides={{
                        Root: {
                          style: {
                            marginBottom: SPACE[8],
                            backgroundColor: theme.colors.backgroundSecondary,
                            ":hover": {
                              backgroundColor: theme.colors.backgroundTertiary,
                            },
                          },
                        },
                      }}
                      name={name}
                      value={typeof args[name] === "string" ? `${args[name]}` : ""}
                      onChange={(e) => {
                        setDeploymentArg({ key: name, value: e.target.value });
                      }}
                    />
                  )}
                </FormControl>
              );
            })}
          </div>
        ) : (
          <></>
        )}
        <Button
          onClick={() => {
            deploySmartContract();
          }}
          isLoading={pending}
          disabled={!smartAccount || shardId === null || !shardIdIsValid}
        >
          Deploy
        </Button>
        {deployError && (
          <ParagraphSmall color={COLORS.red400} marginTop={SPACE[16]}>
            {deployError}
          </ParagraphSmall>
        )}
      </div>
    </>
  );
};
