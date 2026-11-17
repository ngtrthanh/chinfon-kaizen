-- Hardening: immutable audit/history and canonical initiative code.
-- These guards are defence in depth; the application also has no delete or
-- history-edit routes.

CREATE TRIGGER IF NOT EXISTS trg_audit_no_update
BEFORE UPDATE ON audit_log
BEGIN
    SELECT RAISE(ABORT, 'audit_log is append-only');
END;

CREATE TRIGGER IF NOT EXISTS trg_audit_no_delete
BEFORE DELETE ON audit_log
BEGIN
    SELECT RAISE(ABORT, 'audit_log is append-only');
END;

CREATE TRIGGER IF NOT EXISTS trg_history_no_update
BEFORE UPDATE ON initiative_history
BEGIN
    SELECT RAISE(ABORT, 'initiative_history is append-only');
END;

CREATE TRIGGER IF NOT EXISTS trg_history_no_delete
BEFORE DELETE ON initiative_history
BEGIN
    SELECT RAISE(ABORT, 'initiative_history is append-only');
END;

CREATE TRIGGER IF NOT EXISTS trg_initiative_code_immutable
BEFORE UPDATE OF code ON initiative
WHEN OLD.code <> NEW.code
BEGIN
    SELECT RAISE(ABORT, 'initiative code is immutable');
END;

CREATE TRIGGER IF NOT EXISTS trg_initiative_no_delete_submitted
BEFORE DELETE ON initiative
WHEN OLD.submitted_at IS NOT NULL
BEGIN
    SELECT RAISE(ABORT, 'submitted initiative cannot be hard-deleted');
END;

CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_log(actor);
CREATE INDEX IF NOT EXISTS idx_review_reviewer ON initiative_review(reviewer_employee_no);
CREATE INDEX IF NOT EXISTS idx_participant_employee ON initiative_participant(employee_no);
