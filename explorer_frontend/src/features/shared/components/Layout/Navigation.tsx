import { useStyletron } from "styletron-react";
import { Link } from "atomic-router-react";
import { styles } from "./styles";
import { LabelSmall } from "@nilfoundation/ui-kit";
import { getRuntimeConfigOrThrow } from "../../../runtime-config";

const rtc = getRuntimeConfigOrThrow();

export const Navigation = () => {
  const [css] = useStyletron();

  return (
    <ul className={css(styles.navigation)}>
      {config.map(({ title, to }) => (
        <li key={title} className={css(styles.navItem)}>
          <Link to={to} className={css(styles.navLink)}>
            <LabelSmall>{title}</LabelSmall>
          </Link>
        </li>
      ))}
    </ul>
  );
};

const config = [
  {
    title: "Documentation",
    to: rtc.DOCUMENTATION_URL,
  },
  {
    title: "GitHub",
    to: rtc.GITHUB_URL,
  },
] as const;
