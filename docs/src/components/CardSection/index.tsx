import type { ReactNode, PropsWithChildren } from "react";
import Link from "@docusaurus/Link";
import clsx from "clsx";

export function CardSection({
  id,
  title,
  children,
  description,
  className,
  hasSubSections = false,
  HeadingTag = "h3",
}: {
  id?: string;
  title: string;
  children: ReactNode;
  description?: ReactNode;
  hasSubSections?: boolean;
  HeadingTag?: keyof JSX.IntrinsicElements;
  className?: string;
}) {
  return (
    <div className={clsx("homepage-section", hasSubSections && "has-sub-sections", className)}>
      {title && <HeadingTag id={id ?? title}>{title}</HeadingTag>}
      {description && <p className="section-description">{description}</p>}
      <div className="section-content">{children}</div>
    </div>
  );
}

export function Card({
  id,
  title,
  description,
  to,
  tag,
  className,
}: PropsWithChildren<{
  id?: string;
  title: string;
  description?: string;
  to: string;
  tag?: {
    label: string;
    color: string;
    description: string;
  };
  className?: string;
}>) {
  const label = `Index page: ${title}`;
  return (
    <Link
      to={to}
      className={clsx("homepage-card", className)}
      data-goatcounter-click={to}
      data-goatcounter-title={label}
    >
      <div className="card-content">
        <div className="title" id={id && title}>
          {title}
        </div>
        {description && <div className="description">{description}</div>}
      </div>
      {tag && (
        <div className="tag absolute right-0 top-0 h-16 w-16">
          <span
            className="absolute right-[-28px] top-[-2px] w-[80px] rotate-45 transform bg-gray-600 py-1 text-center font-semibold text-white"
            style={{ backgroundColor: tag.color }}
            title={tag.description}
          >
            {tag.label}
          </span>
        </div>
      )}
    </Link>
  );
}
