import IconLinkItem from "../IconLinkItem";
import DiscordIcon from '../../../../static/img/footer/discord.svg';
import TGIcon from '@site/static/img/footer/telegram.svg';
import GHIcon from '@site/static/img/footer/github.svg';

export default function IconLinkItems() {
    const GoToCommunityLink = (Url) => () => {
        window.open(Url);
    };

    return (
        <div className="communityLinksContainer">
            <IconLinkItem IconComponent={DiscordIcon} onIconClick={GoToCommunityLink('https://discord.gg/KmTAEjbmM3')}></IconLinkItem>
            <IconLinkItem IconComponent={TGIcon} onIconClick={GoToCommunityLink('https://t.me/nilfoundation')}></IconLinkItem>
            <IconLinkItem IconComponent={GHIcon} onIconClick={GoToCommunityLink('https://github.com/nilfoundation')}></IconLinkItem>
        </div>
    );
}
