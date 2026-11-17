-- Staff onboarding + production/demo switch.
-- PIN bootstrap is not stored: first login uses the last 5 digits of mobile_num,
-- then the user is forced to set a strong password.

ALTER TABLE md_employee ADD COLUMN email TEXT NOT NULL DEFAULT '';
ALTER TABLE md_employee ADD COLUMN mobile_num TEXT NOT NULL DEFAULT '';
ALTER TABLE md_employee ADD COLUMN report_to TEXT;

ALTER TABLE app_user ADD COLUMN must_change_password INTEGER NOT NULL DEFAULT 0 CHECK(must_change_password IN (0,1));

ALTER TABLE initiative ADD COLUMN is_demo INTEGER NOT NULL DEFAULT 0 CHECK(is_demo IN (0,1));

CREATE TABLE IF NOT EXISTS app_setting (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT OR IGNORE INTO app_setting(key, value) VALUES('demo_mode', '1');

CREATE INDEX IF NOT EXISTS idx_employee_department ON md_employee(department_code);
CREATE INDEX IF NOT EXISTS idx_employee_report_to ON md_employee(report_to);
CREATE INDEX IF NOT EXISTS idx_initiative_demo ON initiative(is_demo);
