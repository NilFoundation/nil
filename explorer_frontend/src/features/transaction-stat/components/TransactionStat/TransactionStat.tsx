import { ParagraphLarge } from "baseui/typography";
import { useStyletron } from "styletron-react";
import { styles } from "./styles";
import { ErrorBoundary } from "react-error-boundary";
import { Chart } from "./Chart";
import { InfoContainer } from "../../../shared";

const ErrorView = () => {
  const [css] = useStyletron();

  return (
    <div className={css(styles.errorViewContainer)}>
      <ParagraphLarge>
        An error occurred while displaying the chart. Please try again later.
      </ParagraphLarge>
    </div>
  );
};

export const TransactionStat = () => {
  return (
    <InfoContainer title="Messages">
      <ErrorBoundary fallback={<ErrorView />}>
        <Chart />
      </ErrorBoundary>
    </InfoContainer>
  );
};
