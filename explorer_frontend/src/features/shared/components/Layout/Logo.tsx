import { Link } from "atomic-router-react";
import { useStore } from "effector-react";
import { useStyletron } from "styletron-react";
import { explorerRoute } from "../../../routing/routes/explorerRoute";
import { tutorialWithUrlStringRoute } from "../../../routing/routes/tutorialRoute";
import logo from "./assets/Logo.svg";
import { styles } from "./styles";

export const Logo = () => {
  const [css] = useStyletron();

  const isTutorial = useStore(tutorialWithUrlStringRoute.$isOpened);

  return (
    <Link className={css(styles.logo)} to={explorerRoute}>
      <img src={logo} alt="logo" />
      {isTutorial && <span className={css(styles.tutorialText)}>Tutorials v1.0</span>}
    </Link>
  );
};
