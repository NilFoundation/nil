import { BUTTON_KIND, BUTTON_SIZE, Button, ButtonIcon, SearchIcon } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { expandProperty } from "inline-style-expand-shorthand";
import type { FC } from "react";
import { getRuntimeConfigOrThrow } from "../../runtime-config/getRuntimeConfigOrThrow";
import { StatefulPopover } from "../../shared/components/Popover";
import { useMobile } from "../../shared/hooks/useMobile";
import { CommunicateIcon } from "../assets/CommunicateIcon";
import { DocsIcon } from "../assets/DocsIcon";
import { GithubIcon } from "../assets/GithubIcon";
import { PlaygroundIcon } from "../assets/PlaygroundIcon";
import { ResourcesIcon } from "../assets/ResourcesIcon";
import { SupportIcon } from "../assets/SupportIcon";
import { TelegramIcon } from "../assets/TelegramIcon";
import { TutorialsIcon } from "../assets/TutorialsIcon";

type ResourceProps = {
  resourceName: string;
};

const ResourceIcons = {
  Playground: PlaygroundIcon(),
  Tutorials: TutorialsIcon(),
  Explorer: <SearchIcon size="24px" />,
  Docs: DocsIcon(),
  Support: SupportIcon(),
  Feedback: CommunicateIcon(),
  Github: GithubIcon(),
  Telegram: TelegramIcon(),
};

const { RPC_TELEGRAM_BOT, PLAYGROUND_FEEDBACK_URL } = getRuntimeConfigOrThrow();

const ResourceURLs = {
  Playground: "https://explore.nil.foundation/playground",
  Tutorials: "https://explore.nil.foundation/tutorial/async-call",
  Explorer: "https://explore.nil.foundation",
  Docs: "https://docs.nil.foundation",
  Support: RPC_TELEGRAM_BOT,
  Feedback: PLAYGROUND_FEEDBACK_URL,
  Github: "https://github.com/NilFoundation/nil",
  Telegram: "https://t.me/NilDevBot?start=ref_playground",
};

const Resource: FC<ResourceProps> = ({ resourceName }) => {
  const [css, theme] = useStyletron();
  const [isMobile] = useMobile();

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
              height: "80px",
              width: "90px",
              color: `${theme.colors.resourceTextColor} !important`,
              gap: "12px",
              fontSize: isMobile ? "12px" : "16px",
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
      placement={isMobile ? "bottomRight" : "bottom"}
      content={
        <div
          className={css({
            display: "grid",
            gridTemplateColumns: "repeat(3, 1fr)",
            gridTemplateRows: "repeat(3, 1fr)",
            gap: "8px",
            ...expandProperty("padding", "16px"),
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
