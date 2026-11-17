-- Production mode: never seed demo data by default. Existing demo records
-- (is_demo=1) remain stored for audit but are hidden from every query.
INSERT INTO app_setting(key, value) VALUES('demo_mode', '0')
ON CONFLICT(key) DO UPDATE SET value='0';
