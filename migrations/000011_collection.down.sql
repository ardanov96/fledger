-- Migration 000011_collection DOWN (rollback only).
BEGIN;

DROP TRIGGER IF EXISTS collection_events_apply_to_stop ON collection_events;
DROP TRIGGER IF EXISTS route_stops_recompute_route_total ON route_stops;
DROP FUNCTION IF EXISTS route_stop_apply_collection_event();
DROP FUNCTION IF EXISTS collection_route_recompute_totals();

DROP TABLE IF EXISTS settlements;
DROP TABLE IF EXISTS collection_events;
DROP TABLE IF EXISTS route_stops;
DROP TABLE IF EXISTS collection_routes;

COMMIT;
