-- Hide seeded demo master data from every picker/list while preserving
-- referential integrity for archived demo initiatives.
ALTER TABLE md_department ADD COLUMN is_demo INTEGER NOT NULL DEFAULT 0 CHECK(is_demo IN (0,1));
ALTER TABLE md_employee ADD COLUMN is_demo INTEGER NOT NULL DEFAULT 0 CHECK(is_demo IN (0,1));

UPDATE md_department
SET is_demo = 1
WHERE code IN ('MINE','KILN','GRIND','LOGISTICS','QHSE','SALES','SUPPORT');

UPDATE md_employee
SET is_demo = 1
WHERE employee_no IN (
    'CF-0001','CF-0005','CF-0010','CF-0018','CF-0118','CF-0088',
    'CF-0412','CF-0519','CF-0611','CF-0711','CF-0811','CF-0911'
)
AND NOT EXISTS (
    SELECT 1 FROM app_user u WHERE u.employee_no = md_employee.employee_no
);

CREATE INDEX IF NOT EXISTS idx_department_demo ON md_department(is_demo);
CREATE INDEX IF NOT EXISTS idx_employee_demo ON md_employee(is_demo);
