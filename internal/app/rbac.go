package app

import "strings"

func (u *User) HasRole(role string) bool {
	if u == nil {
		return false
	}
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (u *User) HasPerm(perm string) bool {
	if u == nil || u.Permissions == nil {
		return false
	}
	return u.Permissions[perm]
}

func (u *User) IsAdmin() bool { return u.HasPerm(PermRBACAdmin) }

func (u *User) CanSeeConfidential() bool { return u.HasPerm(PermConfidential) }

// BroadVisibility identifies roles that may see records outside their own
// submissions. A department scope still narrows the result when it is not ALL.
func (u *User) BroadVisibility() bool {
	return u.HasPerm(PermScreen) || u.HasPerm(PermReviewROI) || u.HasPerm(PermApproveFinal) ||
		u.HasPerm(PermPayroll) || u.HasPerm(PermRBACAdmin)
}

func (u *User) ScopeAll() bool {
	return strings.EqualFold(strings.TrimSpace(u.Scope), "ALL")
}

// CanView enforces the cem-auth scope guard. Confidential records have a
// separate gate so no intermediate manager can infer their existence.
func (u *User) CanView(i *Initiative) bool {
	if u == nil || i == nil {
		return false
	}
	if i.IsDemo && !u.DemoMode {
		return false
	}
	// A submitter may always see their own record, including a confidential
	// report they filed. They still cannot see anyone else's Luồng 3 record.
	if i.SubmitterEmployeeNo == u.EmployeeNo {
		return true
	}
	if i.TrackCode == TrackConfidential && !u.CanSeeConfidential() {
		return false
	}
	if !u.BroadVisibility() {
		return false
	}
	if u.ScopeAll() {
		return true
	}
	return i.DepartmentCode == u.Scope
}

func (u *User) CanViewWithParticipants(i *Initiative, participants []Participant) bool {
	if i == nil {
		return false
	}
	if i.IsDemo && !u.DemoMode {
		return false
	}
	if u.CanView(i) {
		return true
	}
	if i.TrackCode == TrackConfidential && !u.CanSeeConfidential() {
		return false
	}
	for _, p := range participants {
		if p.EmployeeNo == u.EmployeeNo {
			return true
		}
	}
	return false
}

// CanAct checks both stage permission and record scope. ADMIN is the only role
// that can bypass the scope guard, and only because it is explicitly configured
// with scope ALL at user creation.
func (u *User) CanAct(i *Initiative, perm string) bool {
	if !u.CanView(i) || !u.HasPerm(perm) {
		return false
	}
	if u.ScopeAll() {
		return i.RecordStatus == StatusOpen
	}
	return i.DepartmentCode == u.Scope && i.RecordStatus == StatusOpen
}

func (u *User) SelfReviewBlocked(i *Initiative) bool {
	if i == nil || u == nil {
		return false
	}
	return i.SubmitterEmployeeNo == u.EmployeeNo && !u.HasRole(RoleCEOAudit) && !u.IsAdmin()
}

// VisibilityClause returns a SQL fragment over alias i and its bind arguments.
// It is deliberately built from controlled role/permission data, never from
// request input.
func (u *User) VisibilityClause() (string, []any) {
	var clauses []string
	var args []any

	if !u.DemoMode {
		clauses = append(clauses, "i.is_demo = 0")
	}

	if !u.CanSeeConfidential() {
		clauses = append(clauses, "(i.track_code <> '"+TrackConfidential+"' OR i.submitter_employee_no = ?)")
		args = append(args, u.EmployeeNo)
	}

	if u.BroadVisibility() {
		if !u.ScopeAll() {
			clauses = append(clauses, "i.department_code = ?")
			args = append(args, u.Scope)
		}
	} else {
		clauses = append(clauses, `(i.submitter_employee_no = ? OR EXISTS (
			SELECT 1 FROM initiative_participant p
			WHERE p.initiative_id = i.id AND p.employee_no = ?))`)
		args = append(args, u.EmployeeNo, u.EmployeeNo)
	}

	if len(clauses) == 0 {
		return "", nil
	}
	return strings.Join(clauses, " AND "), args
}
