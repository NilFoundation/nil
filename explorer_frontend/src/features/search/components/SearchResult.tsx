import { useStyletron } from "baseui";
import { type SearchItem, unfocus } from "../models/model";
import { Link } from "atomic-router-react";
import { ParagraphSmall, ParagraphXSmall } from "baseui/typography";
import { PRIMITIVE_COLORS } from "@nilfoundation/ui-kit";

type SearchResultProps = {
  items: SearchItem[];
};

export const SearchResult = ({ items }: SearchResultProps) => {
  const [css] = useStyletron();

  const textClassName = css({
    whiteSpace: "nowrap",
    overflow: "hidden",
    textOverflow: "ellipsis",
  });

  return (
    <div
      className={css({
        backgroundColor: PRIMITIVE_COLORS.gray800,
      })}
    >
      {items.map((item) => {
        return (
          <div key={item.type + item.label}>
            <Link
              to={item.route}
              params={item.params}
              onMouseDown={() => {
                item.route.open(item.params);
                unfocus();
              }}
              className={css({
                padding: "12px 48px",
                cursor: "pointer",
                display: "block",
              })}
            >
              <ParagraphXSmall
                className={css({
                  whiteSpace: "nowrap",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  textTransform: "capitalize",
                })}
              >
                {item.type}
              </ParagraphXSmall>
              <ParagraphSmall className={textClassName}>{item.label}</ParagraphSmall>
            </Link>
          </div>
        );
      })}
      {items.length === 0 && (
        <div
          className={css({
            padding: "12px 48px",
            cursor: "pointer",
            display: "block",
          })}
        >
          <ParagraphSmall className={textClassName}>No results found</ParagraphSmall>
        </div>
      )}
    </div>
  );
};
