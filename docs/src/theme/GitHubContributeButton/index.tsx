import styles from './styles.module.css';
import React from 'react';
import GitHubLogo from '@site/static/img/footer/githubNavbar.svg';

export default function GitHubContributeButton({ className }): JSX.Element {
    const GoToGitHub = () => {
        window.open('https://github.com/NilFoundation/docs.nil.foundation');
    };
    return (
        <div className={`${styles.circle} ${className} ${styles.navbarGitHubContributeButton}`}>
            <div onClick={GoToGitHub} className={styles.gitHubLogo}>
                <GitHubLogo />
            </div>
        </div >

    );
}