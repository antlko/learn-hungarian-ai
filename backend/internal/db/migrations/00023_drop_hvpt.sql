-- +goose Up

-- The ear trainer («Слух», HVPT) was removed (ADR-084). Its per-learner state
-- goes with it. Irreversible by design: the learner asked for the feature to
-- be gone completely. Clips under <dataDir>/media/hvpt are not touched here.
DROP TABLE IF EXISTS hvpt_trial;
DROP TABLE IF EXISTS hvpt_contrast_state;
DELETE FROM app_meta WHERE key = 'hvpt_steps_reconciled';

-- +goose Down

-- Not reversible: the dropped HVPT history is not recoverable.
SELECT 1;
