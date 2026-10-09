-- =============================================================================
-- 000002_seed_demo.sql
-- Sprint 1 — minimal demo seed: 2 vehicles, 2 drivers, 3 DO items.
-- Idempotent: ON CONFLICT DO NOTHING for plate/phone uniqueness.
-- =============================================================================

INSERT INTO fleet_vehicles (tenant_id, plate_number, vehicle_type, brand_model, capacity_kg, status)
VALUES
  ('00000000-0000-0000-0000-000000000001', 'B 9123 FMC', 'CDE_BOX', 'Isuzu Traga Box', 1500.00, 'AVAILABLE'),
  ('00000000-0000-0000-0000-000000000001', 'B 9456 FMC', 'CDD_BOX', 'Mitsubishi Colt Diesel', 4000.00, 'AVAILABLE')
ON CONFLICT (plate_number) DO NOTHING;

INSERT INTO fleet_drivers (tenant_id, full_name, phone_number, license_number, status)
VALUES
  ('00000000-0000-0000-0000-000000000001', 'Budi Santoso', '081298765001', 'SIM-B1-881230', 'ACTIVE'),
  ('00000000-0000-0000-0000-000000000001', 'Agus Pratama', '081298765002', 'SIM-B1-881231', 'ACTIVE')
ON CONFLICT (phone_number) DO NOTHING;

INSERT INTO fleet_schema_migrations (version, description)
VALUES (2, 'seed demo vehicles + drivers')
ON CONFLICT (version) DO NOTHING;