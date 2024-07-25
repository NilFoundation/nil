import React, { useState, useEffect } from 'react';
import ReactMarkdown from 'react-markdown';

export default function Crypto3ModuleSubPage({ Url }) {
    const [pageText, setPageText] = useState('');

    const FixPageSyntax = (text) => {
        const hintRe = /\{\% hint style="([^"]+)" \%\}([\s\S]+?)\{\% endhint \%\}/g;
        const headingRe = / \#{1,}\{[^}]*\}/g;
        const linkRe = /@ref \w+_(\w+)/g;
        const descriptionRe = /---[\s\S]+?---\s*/g;


        return text
            .replaceAll(descriptionRe, '')
            .replaceAll('@tableofcontents', '')
            .replaceAll(headingRe, '')
            .replaceAll(linkRe, "'$1'")
            .replaceAll('\.md', '')
            .replaceAll(hintRe, ":::$1\n\n$2\n\n:::");

    };

    useEffect(() => {
        fetch(Url)
            .then(response => response.text())
            .then(text => {
                setPageText(FixPageSyntax(text));
            })
            .catch(error => console.error('Error fetching page text:', error));
    }, [Url]);

    return (
        <ReactMarkdown>
            {pageText}
        </ReactMarkdown>
    );
}
