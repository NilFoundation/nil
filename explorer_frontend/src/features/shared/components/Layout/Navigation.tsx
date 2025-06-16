import { COLORS, LabelSmall } from "@nilfoundation/ui-kit";
import { Link, useRouter } from "atomic-router-react";
import { useUnit } from "effector-react";
import { useStyletron } from "styletron-react";
import { playgroundRoute } from "../../../routing/routes/playgroundRoute";
import { tutorialWithUrlStringRoute } from "../../../routing/routes/tutorialRoute";
import { getRuntimeConfigOrThrow } from "../../../runtime-config";
import { styles } from "./styles";

const rtc = getRuntimeConfigOrThrow();

export const Navigation = () => {
  const [css] = useStyletron();
  const router = useRouter();
  const activeRoutes = useUnit(router.$activeRoutes);
  const activeRoute = activeRoutes[0];
  const isPlayground = activeRoute === playgroundRoute;
  const isTutorial = activeRoute === tutorialWithUrlStringRoute;

  const config = [
    {
      title: "Playground",
      to: playgroundRoute,
      isActive: isPlayground,
    },
    {
      title: "Tutorials",
      to: tutorialWithUrlStringRoute,
      isActive: isTutorial,
    },
    {
      title: "Documentation",
      to: rtc.DOCUMENTATION_URL,
      isActive: false,
    },
    {
      title: "GitHub",
      to: rtc.GITHUB_URL,
      isActive: false,
    },
  ] as const;

  return (
    <ul className={css(styles.navigation)}>
      {config.map(({ title, to, isActive }) => (
        <li key={title} className={css(styles.navItem)}>
          <Link
            to={to}
            className={css(styles.navLink)}
            params={title === "Tutorials" ? { urlSlug: "async-call" } : undefined}
          >
            <LabelSmall color={isActive ? COLORS.gray50 : COLORS.gray200}>{title}</LabelSmall>
          </Link>
        </li>
      ))}
    </ul>
  );
};
