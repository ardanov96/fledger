-- =============================================================================
-- Migration: 000011_collection
-- Description: Collection routes + route stops + collection events + settlements
--              (Sprint 11 — Portfolio Sprint 4 / Fase 8 partial).
--
-- Domain model:
--   * collection_routes — daily plan by a sales rep (1 per rep per day)
--   * route_stops       — customer stops in a route (sequence-ordered)
--   * collection_events — actual collection events at a stop (immutable)
--   * settlements       — sales rep's end-of-day deposit with discrepancy
--
-- Sprint 11 deliverables:
--   1. Plan a route (auto-suggest stops from outstanding invoices, FIFO by due_date)
--   2. Record visits + collection events (immutable, append-only)
--   3. Close stops + complete route
--   4. Settle route (sales rep deposits total collected vs expected)
--   5. Supervisor approve settlement (any discrepancy needs approval)
--
-- All amounts are BIGINT (minor units, e.g. IDR sen = 1/100 IDR).
-- =============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- collection_routes — one daily route per sales rep per tenant
-- ---------------------------------------------------------------------------
CREATE TABLE collection_routes (
    id                      UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID            NOT NULL,
    sales_rep_id            UUID            NOT NULL REFERENCES accounts(id),
    route_date              DATE            NOT NULL,
    status                  TEXT            NOT NULL DEFAULT 'planned',

    -- Auto-computed totals (updated by trigger when stops/events change).
    total_planned_minor     BIGINT          NOT NULL DEFAULT 0,
    total_collected_minor   BIGINT          NOT NULL DEFAULT 0,

    created_at              TIMESTAMPTZ     NOT NULL DEFAULT now(),
    started_at              TIMESTAMPTZ,
    completed_at            TIMESTAMPTZ,
    settled_at              TIMESTAMPTZ,

    metadata                JSONB           NOT NULL DEFAULT '{}',

    CONSTRAINT collection_routes_status_check CHECK (
        status IN ('planned', 'in_progress', 'completed', 'settled', 'cancelled')
    ),

    -- 1 route per sales rep per day per tenant (idempotent re-planning).
    CONSTRAINT collection_routes_unique_per_day UNIQUE (tenant_id, sales_rep_id, route_date)
);

CREATE INDEX collection_routes_tenant_rep_date_idx
    ON collection_routes(tenant_id, sales_rep_id, route_date DESC);

CREATE INDEX collection_routes_status_idx
    ON collection_routes(status, route_date DESC)
    WHERE status IN ('planned', 'in_progress');

CREATE INDEX collection_routes_date_idx
    ON collection_routes(route_date)
    WHERE status IN ('in_progress', 'completed');


-- ---------------------------------------------------------------------------
-- route_stops — customer visits within a route
-- ---------------------------------------------------------------------------
CREATE TABLE route_stops (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id                    UUID        NOT NULL REFERENCES collection_routes(id) ON DELETE CASCADE,
    customer_id                 UUID        NOT NULL REFERENCES accounts(id),
    sequence                    INT         NOT NULL CHECK (sequence > 0),
    planned_invoice_ids         UUID[]      NOT NULL DEFAULT '{}',
    actual_collection_minor     BIGINT      NOT NULL DEFAULT 0,
    status                      TEXT        NOT NULL DEFAULT 'pending',
    visited_at                  TIMESTAMPTZ,
    closed_at                   TIMESTAMPTZ,
    notes                       TEXT,

    -- Optional geolocation columns (nullable, not enforced in Sprint 11).
    latitude                    DOUBLE PRECISION,
    longitude                   DOUBLE PRECISION,

    CONSTRAINT route_stops_status_check CHECK (
        status IN ('pending', 'visited', 'skipped', 'closed')
    ),

    -- One stop per customer per route.
    CONSTRAINT route_stops_unique_customer UNIQUE (route_id, customer_id),
    -- Sequence numbers are unique within a route.
    CONSTRAINT route_stops_unique_sequence  UNIQUE (route_id, sequence)
);

CREATE INDEX route_stops_route_idx ON route_stops(route_id);
CREATE INDEX route_stops_customer_idx ON route_stops(customer_id);
CREATE INDEX route_stops_status_idx
    ON route_stops(status)
    WHERE status IN ('pending', 'visited');


-- ---------------------------------------------------------------------------
-- collection_events — immutable record of each collection at a stop
-- ---------------------------------------------------------------------------
CREATE TABLE collection_events (
    id                  UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    stop_id             UUID            NOT NULL REFERENCES route_stops(id) ON DELETE CASCADE,
    amount_minor        BIGINT          NOT NULL CHECK (amount_minor > 0),
    payment_method      TEXT            NOT NULL,
    reference           TEXT,
    collected_at        TIMESTAMPTZ     NOT NULL DEFAULT now(),
    notes               TEXT,
    recorded_by         UUID            NOT NULL,

    CONSTRAINT collection_events_payment_method_check CHECK (
        payment_method IN ('cash', 'qris', 'transfer', 'cheque')
    )
);

CREATE INDEX collection_events_stop_idx ON collection_events(stop_id);
CREATE INDEX collection_events_collected_at_idx ON collection_events(collected_at DESC);


-- ---------------------------------------------------------------------------
-- settlements — sales rep's end-of-day deposit
-- ---------------------------------------------------------------------------
CREATE TABLE settlements (
    id                          UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id                    UUID            NOT NULL UNIQUE REFERENCES collection_routes(id),
    expected_amount_minor       BIGINT          NOT NULL,
    settled_amount_minor        BIGINT          NOT NULL,
    discrepancy_minor           BIGINT          NOT NULL, -- settled - expected
    status                      TEXT            NOT NULL DEFAULT 'pending',
    submitted_at                TIMESTAMPTZ,
    approved_at                 TIMESTAMPTZ,
    approved_by                 UUID,
    notes                       TEXT,

    CONSTRAINT settlements_status_check CHECK (
        status IN ('pending', 'approved', 'disputed', 'rejected')
    ),

    CONSTRAINT settlements_amounts_positive CHECK (
        expected_amount_minor >= 0 AND settled_amount_minor >= 0
    )
);

CREATE INDEX settlements_status_idx
    ON settlements(status, submitted_at DESC)
    WHERE status IN ('pending', 'disputed');

CREATE INDEX settlements_route_idx ON settlements(route_id);


-- ---------------------------------------------------------------------------
-- Trigger: keep route_stops.actual_collection_minor in sync with events
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION route_stop_apply_collection_event()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE route_stops
        SET actual_collection_minor = actual_collection_minor + NEW.amount_minor
        WHERE id = NEW.stop_id;
        RETURN NEW;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER collection_events_apply_to_stop
AFTER INSERT ON collection_events
FOR EACH ROW
EXECUTE FUNCTION route_stop_apply_collection_event();


-- ---------------------------------------------------------------------------
-- Trigger: keep collection_routes.total_collected_minor in sync with stops
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION collection_route_recompute_totals()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE collection_routes
    SET total_collected_minor = COALESCE((
        SELECT SUM(actual_collection_minor)
        FROM route_stops
        WHERE route_id = COALESCE(NEW.route_id, OLD.route_id)
    ), 0)
    WHERE id = COALESCE(NEW.route_id, OLD.route_id);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER route_stops_recompute_route_total
AFTER INSERT OR UPDATE OR DELETE ON route_stops
FOR EACH ROW
EXECUTE FUNCTION collection_route_recompute_totals();


COMMENT ON TABLE collection_routes IS
    'Daily collection route plan by a sales rep (Portfolio Sprint 4).';
COMMENT ON TABLE route_stops IS
    'Customer stops within a route — sequence-ordered visit plan.';
COMMENT ON TABLE collection_events IS
    'Immutable record of each collection event at a stop (append-only audit trail).';
COMMENT ON TABLE settlements IS
    'Sales rep end-of-day deposit — discrepancy must be approved by supervisor.';

COMMIT;

-- =============================================================================
-- DOWN (rollback only, not auto-applied)
-- =============================================================================
-- DROP TABLE IF EXISTS settlements;
-- DROP TABLE IF EXISTS collection_events;
-- DROP TABLE IF EXISTS route_stops;
-- DROP TABLE IF EXISTS collection_routes;
-- DROP FUNCTION IF EXISTS route_stop_apply_collection_event();
-- DROP FUNCTION IF EXISTS collection_route_recompute_totals();
