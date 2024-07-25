import React from 'react';
import styles from './styles.module.css';

export default function IconLinkItem({ IconComponent, onIconClick }) {
    return (
        <div onClick={onIconClick}>
            <div className={styles.socialButton}>
                <IconComponent></IconComponent>
            </div>
        </div>
    );
}