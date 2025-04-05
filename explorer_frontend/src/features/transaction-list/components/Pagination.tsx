import { BUTTON_KIND, BUTTON_SIZE, ButtonIcon, SPACE } from "@nilfoundation/ui-kit";
import { Button } from "baseui/button";
import { useStyletron } from "styletron-react";
import { PaginationLeftArrow, PaginationRightArrow, useMobile } from "../../shared";

interface PaginationProps {
  currentPage: number;
  totalPages: number;
  onPageChange: (page: number) => void;
}

export const Paginate: React.FC<PaginationProps> = ({ currentPage, totalPages, onPageChange }) => {
  const [css] = useStyletron();
  const [isMobile] = useMobile();

  const getPageNumbers = ({
    currentPage,
    totalPages,
  }: {
    currentPage: number;
    totalPages: number;
  }) => {
    const pages = [];
    pages.push(1);

    if (currentPage > 7) {
      if (currentPage > 7) {
        pages.push("...");
      }
    }

    if (currentPage !== 1 && currentPage !== totalPages) {
      pages.push(currentPage);
    }

    if (currentPage + 1 < totalPages) {
      pages.push(currentPage + 1);

      if (currentPage + 2 < totalPages) {
        pages.push("...");
      }
    }

    if (totalPages > 1) {
      pages.push(totalPages);
    }

    return pages;
  };

  const pageNumbers = getPageNumbers({
    currentPage,
    totalPages,
  });

  return (
    <div
      className={css({
        display: "flex",
        alignItems: "center",
        marginBlockStart: SPACE[24],
      })}
    >
      <ButtonIcon
        kind={BUTTON_KIND.tertiary}
        size={BUTTON_SIZE.default}
        className={css({
          width: "2.3rem",
          height: "2rem",
          background: "#2F2F2F",
          color: "#BDBDBD",
        })}
        onClick={() => currentPage > 1 && onPageChange(currentPage - 1)}
        disabled={currentPage === 1}
        icon={<PaginationLeftArrow />}
      />
      <div
        className={css({
          marginInline: SPACE[8],
          display: "flex",
          gap: isMobile ? ".5rem" : ".5rem",
        })}
      >
        {pageNumbers.map((page) =>
          typeof page === "number" ? (
            <Button
              key={page}
              kind={page === currentPage ? BUTTON_KIND.primary : BUTTON_KIND.tertiary}
              size={BUTTON_SIZE.default}
              className={css({
                width: "2.3rem",
                height: "2rem",
                background: "#2F2F2F",
              })}
              onClick={() => onPageChange(page)}
            >
              {page}
            </Button>
          ) : (
            <span key={page} className={css({ alignSelf: "center" })}>
              {page}
            </span>
          ),
        )}
      </div>
      <ButtonIcon
        kind={BUTTON_KIND.tertiary}
        size={BUTTON_SIZE.default}
        className={css({
          width: "2.3rem",
          height: "2rem",
          background: "#2F2F2F",
          color: "#BDBDBD",
        })}
        onClick={() => currentPage < totalPages && onPageChange(currentPage + 1)}
        disabled={currentPage === totalPages}
        icon={<PaginationRightArrow />}
      />
    </div>
  );
};
