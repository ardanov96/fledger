-- =============================================================================
-- Migration: 000008_period_close (Fase 1A)
-- Adds DB-level triggers that reject new transactions/entries on closed
-- periods. closing flow: trial balance validate → UPDATE status='closed'.
-- =============================================================================

BEGIN;

-- Trigger 1: reject new transactions in closed periods
CREATE OR REPLACE FUNCTION prevent_posted_entry_in_closed_period()
RETURNS TRIGGER AS $$
DECLARE
    period_status TEXT;
BEGIN
    SELECT status INTO period_status FROM accounting_periods WHERE id = NEW.period_id;
    IF period_status IS NULL THEN
        RAISE EXCEPTION 'transaction references unknown period %', NEW.period_id;
    END IF;
    IF period_status NOT IN ('open', 'closing') THEN
        RAISE EXCEPTION 'cannot post to period % (status=%)', NEW.period_id, period_status;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER no_post_to_closed_period
    BEFORE INSERT ON transactions
    FOR EACH ROW EXECUTE FUNCTION prevent_posted_entry_in_closed_period();

-- Trigger 2: same check for direct entries insert
CREATE OR REPLACE FUNCTION prevent_entry_in_closed_period()
RETURNS TRIGGER AS $$
DECLARE
    period_status TEXT;
BEGIN
    SELECT ap.status INTO period_status
    FROM transactions t JOIN accounting_periods ap ON ap.id = t.period_id
    WHERE t.id = NEW.transaction_id;
    IF period_status IS NULL THEN
        RAISE EXCEPTION 'entry references unknown transaction/period';
    END IF;
    IF period_status NOT IN ('open', 'closing') THEN
        RAISE EXCEPTION 'cannot post entry to closed period (status=%)', period_status;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER no_entry_in_closed_period
    BEFORE INSERT ON ledger_entries
    FOR EACH ROW EXECUTE FUNCTION prevent_entry_in_closed_period();

-- Index for fast status lookup during close validation
CREATE INDEX IF NOT EXISTS accounting_periods_tenant_status_idx
    ON accounting_periods(tenant_id, status);

COMMIT;
<task_progress>- [x] Migration 000008 written (triggers + index)
- [ ] Add Period domain types
- [ ] Add PeriodCloser use case
- [ ] Add period_repo
- [ ] Add period handlers + wire
- [ ] Verify build
