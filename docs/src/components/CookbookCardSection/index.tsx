import type { ReactNode, PropsWithChildren } from "react";
import Link from "@docusaurus/Link";
import styles from "./styles.module.css";
import Timer from "@site/static/img/timer.png";
import WhiteTimer from "@site/static/img/timerWhite.png";
import { useColorMode } from "@docusaurus/theme-common";

type CardBadge = {
  label: string;
};

type CardBadgeProp = CardBadge | CardBadge[];

type BadgesProps = {
  badges: CardBadgeProp;
};

export function CookbookCardSection({
  id,
  title,
  children,
  description,
  HeadingTag = "h3",
}: {
  id?: string;
  title: string;
  children: ReactNode;
  description?: ReactNode;
  HeadingTag?: keyof JSX.IntrinsicElements;
  className?: string;
}) {
  return (
    <div className={styles.cookbookSection}>
      {title && <HeadingTag id={id ?? title}>{title}</HeadingTag>}
      {description && <p className="cookbook-section-description">{description}</p>}
      <div className={styles.cookbookSectionContent}>{children}</div>
    </div>
  );
}

export function CookbookCard({
  id,
  title,
  description,
  to,
  tag,
  badges,
}: PropsWithChildren<{
  id?: string;
  title: string;
  description?: string;
  to: string;
  tag?: {
    label: string;
    description: string;
  };
  className?: string;
  badges: BadgesProps;
}>) {
  const { colorMode } = useColorMode();
  const label = `Cookbook: ${title}`;
  return (
    <Link
      to={to}
      className={styles.cookbookCard}
      data-goatcounter-click={to}
      data-goatcounter-title={label}
    >
      <div className={styles.cookbookCardContent}>
        <div className={styles.cookbookCardContentTitle} id={id && title}>
          {title}
        </div>
        {description && <div className={styles.cookbookCardContentDescription}>{description}</div>}
      </div>
      {tag && (
        <div className="tag absolute right-0 top-0 h-16 w-16">
          <span
            className={`${styles.cookbookCardTag} absolute right-[-28px] top-[-2px] w-[80px] rotate-45 transform py-1 text-center font-semibold`}
            title={tag.description}
          >
            <img
              src={colorMode === "dark" ? WhiteTimer : Timer}
              alt="Timer"
              className={styles.timerImage}
            />
            {tag.label}
          </span>
        </div>
      )}
    </Link>
  );
}
