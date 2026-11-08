-- +goose Up

-- Reading time joins the daily ledger.
--
-- The reader was the one loop that reported NOTHING. An hour with a text moved
-- no counter, so «21 / 90 мин» sat still and the day's obligation went unmet
-- while the learner had been working the whole time. `active_ms` is what the
-- streak reads (stats.Day.Met), and reading now adds to it like listening does.
--
-- `read_s` beside it, mirroring `listen_s`, because the total alone cannot say
-- WHERE an hour went — and «куда ушло время» is exactly what the statistics
-- screen is for. Seconds, clamped per report the same way listening is
-- (stats.RecordReading), so a tab left open overnight cannot claim a night's
-- study.
ALTER TABLE daily_log ADD COLUMN read_s REAL NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE daily_log DROP COLUMN read_s;
