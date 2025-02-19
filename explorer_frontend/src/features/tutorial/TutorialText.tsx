import {
  ArrowUpIcon,
  BUTTON_KIND,
  BUTTON_SIZE,
  Button,
  COLORS,
  Card,
  CodeField,
  HeadingLarge,
  HeadingMedium,
  LabelMedium,
  ListItem,
  ParagraphMedium,
  SPACE,
} from "@nilfoundation/ui-kit";
import { Link } from "atomic-router-react";
import { useUnit } from "effector-react";
import Markdown from "react-markdown";
import { useStyletron } from "styletron-react";
import { getMobileStyles } from "../../styleHelpers";
import { сlickOnBackButton } from "../code/model";
import { useMobile } from "../shared";
import { linkStyles } from "../shared/components/Link";
import { $tutorial } from "./model";

export const TutorialText = () => {
  const [isMobile] = useMobile();
  const tutorial = useUnit($tutorial);

  const [css] = useStyletron();
  const HeaderOne = (props: any) => (
    <HeadingLarge
      {...props}
      className={css({
        fontSize: "32px",
        fontWeight: 500,
        marginBottom: "12px",
        lineHeight: "40px",
        marginTop: "10px",
      })}
    />
  );

  const HeaderTwo = (props: any) => (
    <HeadingMedium
      {...props}
      className={css({
        fontSize: "24px",
        fontWeight: 400,
        marginBottom: "8px",
        lineHeight: "30px",
        marginTop: "10px",
      })}
    />
  );

  const CustomParagraph = (props: any) => (
    <ParagraphMedium
      {...props}
      className={css({
        lineHeight: "24px",
        marginBottom: "6px",
        marginTop: "12px",
      })}
    />
  );

  const CustomItalics = (props: any) => (
    <span {...props} className={css({ fontStyle: "italic" })} />
  );

  const CustomListItem = (props: any) => (
    <ListItem
      {...props}
      overrides={{
        Content: {
          style: {
            display: "inline",
          },
        },
      }}
    />
  );

  const CustomLink = (props: any) => <Link {...props} to={props.href} style={linkStyles.link} />;
  const CustomCodeField = ({ node, inline, className, children, ...props }: any) => {
    const codeContent = Array.isArray(children) ? children.join("") : children;
    return <CodeField code={codeContent} {...props} />;
  };

  return (
    <Card
      overrides={{
        Root: {
          style: {
            maxWidth: isMobile ? "calc(100vw - 20px)" : "none",
            width: isMobile ? "100%" : "none",
            height: "91%",
            backgroundColor: COLORS.gray900,
            paddingRight: "0",
            paddingLeft: "0",
            flexDirection: "column",
            paddingBottom: "24px",
          },
        },
        Contents: {
          style: {
            height: "100%",
            maxWidth: "none",
            width: "100%",
            paddingRight: "24px",
            paddingLeft: "24px",
            overflow: "auto",
            overscrollBehavior: "contain",
            display: "flex",
            flexDirection: "column",
            ...getMobileStyles({
              height: "calc(100vh - 154px)",
            }),
          },
        },
        Body: {
          style: {
            height: "auto",
            width: "100%",
            maxWidth: "none",
          },
        },
      }}
    >
      {isMobile && (
        <div
          className={css({
            display: "flex",
            gap: "12px",
            marginBottom: SPACE[12],
            alignItems: "center",
          })}
        >
          <Button
            className={css({
              width: "32px",
              height: "32px",
            })}
            overrides={{
              Root: {
                style: {
                  paddingLeft: 0,
                  paddingRight: 0,
                },
              },
            }}
            kind={BUTTON_KIND.secondary}
            size={BUTTON_SIZE.compact}
            onClick={() => сlickOnBackButton()}
          >
            <ArrowUpIcon
              size={12}
              className={css({
                transform: "rotate(-90deg)",
              })}
            />
          </Button>
          <LabelMedium color={COLORS.gray50}>Tutorial</LabelMedium>
        </div>
      )}
      <Markdown
        components={{
          h1: HeaderOne,
          h2: HeaderTwo,
          p: CustomParagraph,
          li: CustomListItem,
          code: CustomCodeField,
          em: CustomItalics,
          a: CustomLink,
        }}
      >
        {tutorial.text}
      </Markdown>
    </Card>
  );
};
