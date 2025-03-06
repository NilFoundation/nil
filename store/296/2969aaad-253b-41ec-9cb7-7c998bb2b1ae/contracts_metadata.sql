ATTACH TABLE _ UUID '5e2e9c1b-f228-4bff-a0a8-50ca60d98180'
(
    `address` FixedString(20),
    `version` UInt32,
    `data_json` String,
    `code_hash` FixedString(32),
    `abi` String,
    `source_code` Map(String, String)
)
ENGINE = MergeTree
PRIMARY KEY (address, code_hash)
ORDER BY (address, code_hash)
SETTINGS index_granularity = 8192
