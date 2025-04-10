import { Input, SearchIcon } from "@nilfoundation/ui-kit";
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

const Search = () => {
  const [query, focused, results] = useUnit([$query, $focused, $results]);
  const [css, theme] = useStyletron();

  const isShowResult = focused && query.length > 0;
  return (
    <div
      className={css({
        marginLeft: "32px",
        width: "100%",
        position: "relative",
        zIndex: 2,
        flex: 1,
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
        overrides={{
          Root: {
            style: {
              backgroundColor: theme.colors.inputButtonAndDropdownOverrideBackgroundColor,
              ":hover": {
                backgroundColor: theme.colors.inputButtonAndDropdownOverrideBackgroundHoverColor,
              },
              width: "100%",
            },
          },
        }}
      />
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
