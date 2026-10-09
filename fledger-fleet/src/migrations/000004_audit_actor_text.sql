-- =============================================================================
-- 000004_audit_actor_text.sql
--
-- Fix: actor_id columns must be TEXT (not UUID) because callers may pass either
-- a real user UUID OR a service identifier like "dev-user" or "service:...".
-- The DO submit / dispatch handlers always supply a non-empty actor.
-- =============================================================================

ALTER TABLE fleet_delivery_orders
    ALTER COLUMN actor_id TYPE TEXT USING actor_id::text;

ALTER TABLE fleet_trips
    ALTER COLUMN actor_id TYPE TEXT USING actor_id::text;

ALTER TABLE fleet_vehicles
    ALTER COLUMN updated_by TYPE TEXT USING updated_by::text;

ALTER TABLE fleet_do_status_history
    ALTER COLUMN actor_id TYPE TEXT USING actor_id::text;

INSERT INTO fleet_schema_migrations (version, description)
VALUES (4, 'audit actor columns changed from UUID to TEXT (accept service ids)')
ON CONFLICT (version) DO NOTHING;