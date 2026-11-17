-- Personnel grouping (team/shift/unit within a department).
ALTER TABLE md_employee ADD COLUMN work_group TEXT NOT NULL DEFAULT '';
