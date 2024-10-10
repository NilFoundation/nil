import {
  BUTTON_KIND,
  Button,
  COLORS,
  Checkbox,
  FormControl,
  Input,
  ModalBody,
  ModalHeader,
  SPACE,
} from "@nilfoundation/ui-kit";
import { HeadingMedium, ParagraphMedium } from "baseui/typography";
import { useUnit } from "effector-react";
import {
  $assignedAddress,
  $deploymentArgs,
  $shardId,
  assignAdress,
  deploySmartContract,
  deploySmartContractFx,
  setAssignAddress,
  setDeploymentArg,
  setShardId,
} from "./model";
import { useStyletron } from "styletron-react";
import { $constructor } from "./init";

export const DeployForm = () => {
  const [address, args, constuctorAbi, pending, shardId] = useUnit([
    $assignedAddress,
    $deploymentArgs,
    $constructor,
    deploySmartContractFx.pending,
    $shardId,
  ]);
  const [css] = useStyletron();
  return (
    <>
      <ModalHeader>Deploy settings</ModalHeader>
      <ModalBody>
        <div
          className={css({
            flexGrow: 0,
            paddingBottom: SPACE[16],
            borderBottom: `1px solid ${COLORS.gray800}`,
          })}
        >
          <FormControl label="Address" caption="Assign by address">
            <Input
              overrides={{
                Root: {
                  style: {
                    marginBottom: SPACE[8],
                  },
                },
              }}
              name="Addres"
              placeholder="0x..."
              value={address}
              onChange={(e) => {
                setAssignAddress(e.target.value);
              }}
            />
          </FormControl>
          <Button
            kind={BUTTON_KIND.secondary}
            className={css({})}
            onClick={() => {
              assignAdress();
            }}
          >
            Assign
          </Button>
        </div>
        <div
          className={css({
            marginTop: SPACE[16],
          })}
        >
          <FormControl label="Shard id">
            <Input value={shardId} onChange={(e) => setShardId(Number.parseInt(e.target.value))} type="number" />
          </FormControl>
          {constuctorAbi?.inputs.length ? (
            <>
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
            </>
          ) : (
            <ParagraphMedium>No constructor arguments</ParagraphMedium>
          )}
          <Button
            onClick={() => {
              deploySmartContract();
            }}
            isLoading={pending}
          >
            Deploy
          </Button>
        </div>
      </ModalBody>
    </>
  );
};
