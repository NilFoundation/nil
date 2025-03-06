ATTACH TABLE _ UUID 'b2e0384d-7724-47b7-b962-d95df54a2672'
(
    `address` FixedString(20),
    `selector` FixedString(4),
    `name` String,
    `type` String
)
ENGINE = ReplacingMergeTree
PRIMARY KEY (address, selector)
ORDER BY (address, selector)
SETTINGS index_granularity = 8192
