import { BaseProvider } from "baseui";
import { useUnit } from "effector-react";
import type { ReactNode } from "react";
import { tutorialWithUrlStringRoute } from "./features/routing/routes/tutorialRoute";
import { theme, tutorialsTheme } from "./themes";

interface ThemedProviderProps {
  children: ReactNode;
}

export const ThemedProvider = ({ children }: ThemedProviderProps) => {
  const isTutorialPage = useUnit(tutorialWithUrlStringRoute.$isOpened);

  const currentTheme = isTutorialPage ? tutorialsTheme.theme : theme;
  return <BaseProvider theme={currentTheme}>{children}</BaseProvider>;
};
