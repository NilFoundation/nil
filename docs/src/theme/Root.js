import React from 'react';
import CookieConsent, { Cookies, getCookieConsentValue } from "react-cookie-consent";

// Default implementation, that you can customize
export default function Root({ children }) {
    const buttonStyle = {
        display: 'flex',
        flexDirection: 'row',
        alignItems: 'center',
        backgroundColor: '#f2f2f2',
        padding: '0.75rem',
        borderRadius: '5px',
        fontFamily: 'HelveticaNeue'
    }
    const contentStyle = {
        fontFamily: 'HelveticaNeue',
        color: '#f2f2f2'
    }

    const style = {
        backgroundColor: '#181A1B',
    }
    return <>
        <CookieConsent
            location="bottom"
            buttonText="I understand"
            buttonStyle={
                buttonStyle
            }
            contentStyle={
                contentStyle
            }
            overlay={false}
            style={style}
        >This website tracks page views and other usage statistics. No personal information is collected.</CookieConsent >
        {children}
    </>;
}