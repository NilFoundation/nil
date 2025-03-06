import { COLORS, Input, SearchIcon } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { useUnit } from "effector-react";
import {
  $focused,
  $query,
  $results,
  blurSearch,
  clearSearch,
  focusSearch,
  updateSearch,
} from "../models/model";
import { SearchResult } from "./SearchResult";
import { isTutorialPage } from "../../code/model";

const Search = () => {
  const [query, focused, results, isTutorial] = useUnit([$query, $focused, $results, isTutorialPage]);
  const [css] = useStyletron();

  const isShowResult = focused && query.length > 0;
  return (
    <div
      className={css({
        marginLeft: "32px",
        width: "100%",
        position: "relative",
        zIndex: 2,
      })}
    >
      <Input
        placeholder="Search by Address, Transaction Hash, Block Shard ID and Height"
        value={query}
        onFocus={() => {
          focusSearch();
        }}
        onBlur={() => {
          blurSearch();
        }}
        onChange={(e) => {
          updateSearch(e.currentTarget.value);
        }}
        startEnhancer={<SearchIcon />}
        clearable
        onClear={() => {
          clearSearch();
        }}
        {...(isTutorial && {
          overrides: {
            Root: {
              style: {
                backgroundColor: COLORS.blue800,
                ':hover': {
                  backgroundColor: COLORS.blue700,
                }
              }
            }
          }
        })} />
      {isShowResult && (
        <div
          className={css({
            position: "absolute",
            width: "100%",
            top: "100%",
          })}
        >
          <SearchResult items={results} />
        </div>
      )}
    </div>
  );
};

// biome-ignore lint/style/noDefaultExport: <explanation>
export default Search;
