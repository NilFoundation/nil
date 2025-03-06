import { Link } from "atomic-router-react";
import { useStyletron } from "styletron-react";
import { explorerRoute } from "../../../routing/routes/explorerRoute";
import logo from "./assets/Logo.svg";
import { styles } from "./styles";
import { useUnit } from "effector-react";
import { isTutorialPage } from "../../../code/model";

export const Logo = () => {
  const [css] = useStyletron();

  const isTutorial = useUnit(isTutorialPage);

  return (
    <Link className={css(styles.logo)} to={explorerRoute}>
      <img src={logo} alt="logo" />
      {isTutorial && <span className={css(styles.tutorialText)}>Tutorials v1.0</span>}
    </Link>
  );
};
