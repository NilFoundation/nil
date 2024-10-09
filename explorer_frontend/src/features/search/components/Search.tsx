import { Input, SearchIcon } from "@nilfoundation/ui-kit";
import { $focused, $query, $results, blurSearch, clearSearch, focusSearch, updateSearch } from "../models/model";
import { useUnit } from "effector-react";
import { useStyletron } from "baseui";
import { SearchResult } from "./SearchResult";

// implement search login here if needed
const Search = () => {
  const [query, focused, results] = useUnit([$query, $focused, $results]);
  const [css] = useStyletron();

  const isShowResult = focused && query.length > 0;

  return (
    <div
      className={css({
        margin: "0 32px",
        width: "100%",
        position: "relative",
      })}
    >
      <Input
        placeholder="Search by Address, Message Hash, Block Shard ID and Height"
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

export default Search;
