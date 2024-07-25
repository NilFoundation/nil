import React from 'react';
import { useThemeConfig } from '@docusaurus/theme-common';
import FooterLinks from '@theme/Footer/Links';
import FooterLogo from '@theme/Footer/Logo';
import FooterCopyright from '@theme/Footer/Copyright';
import FooterLayout from '@theme/Footer/Layout';
import IconLinkItems from '@theme/Footer/IconLinkItems';
function Footer() {
  const { footer } = useThemeConfig();
  if (!footer) {
    return null;
  }
  const { copyright, logo, style } = footer;
  const iconLinkItems = <IconLinkItems></IconLinkItems>
  return (
    <FooterLayout
      style={style}
      iconLinkItems={iconLinkItems}
      logo={logo && <FooterLogo logo={logo} />}
      copyright={copyright && <FooterCopyright copyright={copyright} />}
    />
  );
}
export default React.memo(Footer);
