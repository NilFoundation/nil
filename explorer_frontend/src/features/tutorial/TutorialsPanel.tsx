import { COLORS, Card, HeadingLarge, StyledBody } from "@nilfoundation/ui-kit";
import { useUnit } from "effector-react";
import { useStyletron } from "styletron-react";
import {
  changeActiveTab,
  openTutorialText,
  setSelectedTutorial,
} from "../../pages/tutorials/model";
import { getMobileStyles } from "../../styleHelpers";
import { tutorialWithUrlStringRoute } from "../routing/routes/tutorialRoute";
import { useMobile } from "../shared/hooks/useMobile";
import { $completedTutorials, type Tutorial, TutorialLevel } from "./model";

const TutorialContainer = ({ tutorial }) => {
  const [isMobile] = useMobile();
  const [css] = useStyletron();

  const tutorialColor = (() => {
    switch (tutorial.level) {
      case TutorialLevel.Easy:
        return COLORS.green200;
      case TutorialLevel.Medium:
        return COLORS.yellow200;
      case TutorialLevel.Hard:
        return COLORS.orange200;
      case TutorialLevel.VeryHard:
        return COLORS.red200;
      default:
        return COLORS.gray200;
    }
  })();
  return (
    // biome-ignore lint/a11y/useKeyWithClickEvents: <explanation>
    <div
      className={css({
        display: "flex",
        flexDirection: "row",
        gap: "12px",
        marginBottom: "12px",
        borderRadius: "8px",
        backgroundColor: COLORS.blue800,
        padding: "16px",
        cursor: "pointer",
        ":hover": {
          backgroundColor: COLORS.blue700,
        },
      })}
      onClick={() => {
        tutorialWithUrlStringRoute.open({ urlSlug: tutorial.urlSlug });
        setSelectedTutorial(tutorial);
        openTutorialText();
        if (isMobile) {
          changeActiveTab("0");
        }
      }}
    >
      <img
        src={tutorial.icon}
        className={css({
          width: "45px",
          height: "45px",
          borderRadius: "8px",
          backgroundColor: COLORS.blue900,
          padding: "8px",
        })}
        aria-label="Tutorial icon"
      />
      <div
        className={css({
          display: "flex",
          flexDirection: "column",
        })}
      >
        <div
          className={css({
            fontSize: "22px",
            fontWeight: 500,
            marginBottom: "14px",
          })}
        >
          {tutorial.title}
        </div>
        <StyledBody
          className={css({
            color: COLORS.gray300,
          })}
        >
          {tutorial.description}
        </StyledBody>
        <div
          className={css({
            display: "flex",
            flexDirection: "row",
            fontSize: "12px",
          })}
        >
          <div
            className={css({
              color: COLORS.gray200,
            })}
          >
            {tutorial.completionTime}
          </div>
          <span style={{ marginRight: "4px", marginLeft: "4px" }}>|</span>
          <div
            className={css({
              color: tutorialColor,
            })}
          >
            {tutorial.level.toString()}
          </div>
        </div>
      </div>
    </div>
  );
};

export const TutorialsPanel = ({ tutorials }) => {
  const completedTutorials = useUnit($completedTutorials);
  const [css] = useStyletron();
  const [isMobile] = useMobile();

  const completedTutorialsList = tutorials.filter((tutorial) =>
    completedTutorials.includes(tutorial.stage),
  );

  const nonCompletedTutorialsList = tutorials.filter(
    (tutorial) => !completedTutorials.includes(tutorial.stage),
  );

  const areCompletedTutorialsEmpty = completedTutorialsList.length === 0;

  return (
    <Card
      overrides={{
        Root: {
          style: {
            maxWidth: isMobile ? "calc(100vw - 20px)" : "none",
            width: isMobile ? "100%" : "none",
            backgroundColor: COLORS.blue900,
            flexDirection: "column",
            padding: "6px",
            height: "calc(100% - 24px)",
          },
        },
        Contents: {
          style: {
            height: "100%",
            maxWidth: "none",
            width: "100%",
            overflow: "auto",
            display: "flex",
            flexDirection: "column",
            ...getMobileStyles({
              height: "calc(100vh - 154px)",
              overflowY: "auto",
            }),
          },
        },
        Body: {
          style: {
            height: "auto",
            width: "100%",
            maxWidth: "none",
            paddingBottom: "20px",
          },
        },
      }}
    >
      {nonCompletedTutorialsList.map((tutorial: Tutorial) => (
        <TutorialContainer key={tutorial.urlSlug} tutorial={tutorial} />
      ))}

      {!areCompletedTutorialsEmpty && (
        <>
          <HeadingLarge
            className={css({
              fontSize: "32px",
              fontWeight: 500,
              marginBottom: "12px",
              lineHeight: "40px",
              marginTop: "10px",
            })}
          >
            Completed tutorials
          </HeadingLarge>

          {completedTutorialsList.map((tutorial) => (
            <TutorialContainer key={tutorial.urlSlug} tutorial={tutorial} />
          ))}
        </>
      )}
    </Card>
  );
};
