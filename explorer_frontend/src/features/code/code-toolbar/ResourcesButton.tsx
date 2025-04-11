import { BUTTON_KIND, BUTTON_SIZE, Button, ButtonIcon } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import type { FC } from "react";
import { StatefulPopover } from "../../shared/components/Popover";
import { useMobile } from "../../shared/hooks/useMobile";
import { DocsIcon } from "../assets/DocsIcon";
import { GithubIcon } from "../assets/GithubIcon";
import { PlaygroundIcon } from "../assets/PlaygroundIcon";
import { ResourcesIcon } from "../assets/ResourcesIcon";
import { SupportIcon as IssuesIcon } from "../assets/SupportIcon";
import { TelegramIcon } from "../assets/TelegramIcon";
import { TutorialsIcon } from "../assets/TutorialsIcon";

type ResourceProps = {
  resourceName: string;
};

const ResourceIcons = {
  Playground: PlaygroundIcon(),
  Tutorials: TutorialsIcon(),
  Docs: DocsIcon(),
  Github: GithubIcon(),
  Issues: IssuesIcon(),
  Telegram: TelegramIcon(),
};

const ResourceURLs = {
  Playground: "https://explore.nil.foundation/playground",
  Tutorials: "https://explore.nil.foundation/tutorial/async-call",
  Docs: "https://docs.nil.foundation",
  Github: "https://github.com/NilFoundation/nil",
  Issues: "https://github.com/NilFoundation/nil/issues",
  Telegram: "https://t.me/NilDevBot?start=ref_playground",
};

const Resource: FC<ResourceProps> = ({ resourceName }) => {
  const [css, theme] = useStyletron();

  const icon: JSX.Element = ResourceIcons[resourceName as keyof typeof ResourceIcons];
  const url: string = ResourceURLs[resourceName as keyof typeof ResourceURLs];
  return (
    <div>
      <Button
        overrides={{
          Root: {
            style: {
              display: "flex",
              alignItems: "center",
              flexDirection: "column",
              backgroundColor: "transparent",
              ":hover": {
                backgroundColor: theme.colors.rpcUrlBackgroundHoverColor,
              },
              height: "100px",
              width: "110px",
              color: `${theme.colors.resourceTextColor} !important`,
              gap: "14px",
            },
          },
        }}
        onClick={() => {
          window.open(url, "_blank");
        }}
        className={css({
          flexShrink: 0,
        })}
      >
        {icon}
        {resourceName}
      </Button>
    </div>
  );
};

export const ResourcesButton = () => {
  const [isMobile] = useMobile();
  const [css, theme] = useStyletron();
  return (
    <StatefulPopover
      popoverMargin={8}
      content={
        <div
          className={css({
            height: "250px",
            width: isMobile ? "380px" : "430px",
            display: "flex",
            alignItems: "center",
            gap: "4px",
            paddingTop: "10px",
            paddingBottom: "10px",
            borderRadius: "8px",
            overflow: "auto",
            flexWrap: "wrap",
            justifyContent: "center",
            alignContent: "center",
            backgroundColor: `${theme.colors.inputButtonAndDropdownOverrideBackgroundColor} !important`,
          })}
        >
          {Object.keys(ResourceIcons).map((resourceName) => (
            <Resource resourceName={resourceName} key={resourceName} />
          ))}
        </div>
      }
    >
      <ButtonIcon
        className={css({
          width: isMobile ? "32px" : "46px",
          height: isMobile ? "32px" : "46px",
          flexShrink: 0,
          backgroundColor: `${theme.colors.inputButtonAndDropdownOverrideBackgroundColor} !important`,
          ":hover": {
            backgroundColor: `${theme.colors.inputButtonAndDropdownOverrideBackgroundHoverColor} !important`,
          },
        })}
        icon={<ResourcesIcon />}
        kind={BUTTON_KIND.secondary}
        size={isMobile ? BUTTON_SIZE.compact : BUTTON_SIZE.large}
      />
    </StatefulPopover>
  );
};
