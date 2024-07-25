import styles from './styles.module.css';
import GitHubLogo from '@site/static/img/footer/github.svg';

export default function GitHubButton({ Url }) {
    const GoToGitHub = (Url) => () => {
        window.open(Url);
    };
    return (
        <div className={styles.gitHubButton} onClick={GoToGitHub(Url)}>
            <GitHubLogo className={styles.gitHubLogo}></GitHubLogo>
            Access the project on GitHub
        </div>
    );
}