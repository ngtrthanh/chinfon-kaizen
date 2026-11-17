package app

import "testing"

func testUser(empNo, scope string, roles ...string) *User {
	perms := map[string]bool{}
	rolePerms := map[string][]string{
		RoleWorker:     {PermCreate},
		RoleSupervisor: {PermCreate, PermScreen},
		RoleManager:    {PermCreate, PermScreen, PermReviewROI},
		RoleCouncil:    {PermCreate, PermScreen, PermReviewROI, PermApproveFinal},
		RoleDirector:   {PermCreate, PermScreen, PermReviewROI, PermApproveFinal},
		RoleHRD:        {PermCreate, PermPayroll},
		RoleCEOAudit:   {PermCreate, PermScreen, PermReviewROI, PermApproveFinal, PermConfidential, PermPayroll},
		RoleAdmin:      {PermCreate, PermScreen, PermReviewROI, PermApproveFinal, PermConfidential, PermPayroll, PermRBACAdmin},
	}
	u := &User{EmployeeNo: empNo, Scope: scope, Roles: roles, Permissions: perms, DemoMode: true}
	for _, r := range roles {
		for _, p := range rolePerms[r] {
			perms[p] = true
		}
	}
	return u
}

func testRecord(track, dept, submitter string) *Initiative {
	return &Initiative{
		TrackCode:           track,
		DepartmentCode:      dept,
		SubmitterEmployeeNo: submitter,
		RecordStatus:        StatusOpen,
		StageCode:           StageReview,
	}
}

func TestCanViewScopeAndConfidentiality(t *testing.T) {
	own := testRecord(TrackStandard, "KILN", "CF-0412")
	otherDept := testRecord(TrackStandard, "GRIND", "CF-0519")
	secret := testRecord(TrackConfidential, "LOGISTICS", "CF-0911")

	worker := testUser("CF-0412", "KILN", RoleWorker)
	if !worker.CanView(own) {
		t.Fatal("worker must see own record")
	}
	if worker.CanView(otherDept) {
		t.Fatal("worker must not see other records")
	}

	supervisor := testUser("CF-0118", "KILN", RoleSupervisor)
	if !supervisor.CanView(own) || supervisor.CanView(otherDept) {
		t.Fatal("supervisor must be limited to KILN")
	}
	if supervisor.CanView(secret) {
		t.Fatal("supervisor must never see confidential track")
	}

	council := testUser("CF-0010", "ALL", RoleCouncil)
	if !council.CanView(otherDept) {
		t.Fatal("council with ALL scope should see standard records")
	}
	if council.CanView(secret) {
		t.Fatal("council must not see confidential track")
	}

	ceo := testUser("CF-0001", "ALL", RoleCEOAudit)
	if !ceo.CanView(secret) {
		t.Fatal("CEO_AUDIT must see confidential track")
	}

	secretSubmitter := testUser("CF-0911", "LOGISTICS", RoleWorker)
	if !secretSubmitter.CanView(secret) {
		t.Fatal("confidential submitter must still track their own report")
	}
	otherWorker := testUser("CF-0412", "KILN", RoleWorker)
	if otherWorker.CanView(secret) {
		t.Fatal("other workers must never see a confidential report")
	}

	admin := testUser("CF-9999", "ALL", RoleAdmin)
	if !admin.CanView(secret) || !admin.CanView(otherDept) {
		t.Fatal("admin must see everything")
	}
}

func TestCanActUsesStagePermissionAndScope(t *testing.T) {
	rec := testRecord(TrackStandard, "KILN", "CF-0412")
	rec.StageCode = StageScreen
	supervisor := testUser("CF-0118", "KILN", RoleSupervisor)
	if !supervisor.CanAct(rec, PermScreen) {
		t.Fatal("supervisor should act on KILN screen stage")
	}
	other := testUser("CF-0218", "GRIND", RoleSupervisor)
	if other.CanAct(rec, PermScreen) {
		t.Fatal("supervisor outside dept must not act")
	}
	if supervisor.CanAct(rec, PermReviewROI) {
		t.Fatal("supervisor lacks review ROI")
	}
	rec.RecordStatus = StatusRejected
	if supervisor.CanAct(rec, PermScreen) {
		t.Fatal("closed/rejected records must not be actionable")
	}
}

func TestSelfReviewGuard(t *testing.T) {
	rec := testRecord(TrackStandard, "KILN", "CF-0018")
	manager := testUser("CF-0018", "KILN", RoleManager)
	if !manager.SelfReviewBlocked(rec) {
		t.Fatal("submitter must not review own record")
	}
	admin := testUser("CF-0018", "ALL", RoleAdmin)
	if admin.SelfReviewBlocked(rec) {
		t.Fatal("admin override is allowed")
	}
}

func TestVisibilityClause(t *testing.T) {
	supervisor := testUser("CF-0118", "KILN", RoleSupervisor)
	where, args := supervisor.VisibilityClause()
	if where == "" || len(args) != 2 || args[0] != "CF-0118" || args[1] != "KILN" {
		t.Fatalf("unexpected clause: %q %#v", where, args)
	}
	worker := testUser("CF-0412", "KILN", RoleWorker)
	where, args = worker.VisibilityClause()
	if len(args) != 3 || args[0] != "CF-0412" {
		t.Fatalf("worker clause must bind own employee number for confidential and own/participant visibility: %q %#v", where, args)
	}
	ceo := testUser("CF-0001", "ALL", RoleCEOAudit)
	where, args = ceo.VisibilityClause()
	if where != "" || len(args) != 0 {
		t.Fatalf("CEO clause should be unrestricted in demo mode, got %q %#v", where, args)
	}
	ceo.DemoMode = false
	where, args = ceo.VisibilityClause()
	if where != "i.is_demo = 0" || len(args) != 0 {
		t.Fatalf("production mode must hide demo records, got %q %#v", where, args)
	}
	demoRec := testRecord(TrackStandard, "KILN", "CF-0412")
	demoRec.IsDemo = true
	ceo.DemoMode = true
	if !ceo.CanView(demoRec) {
		t.Fatal("demo records should be visible while demo mode is on")
	}
	ceo.DemoMode = false
	if ceo.CanView(demoRec) {
		t.Fatal("demo records must be invisible after demo mode is off")
	}
	participant := testUser("CF-0118", "KILN", RoleWorker)
	participant.DemoMode = false
	if participant.CanViewWithParticipants(demoRec, []Participant{{EmployeeNo: "CF-0118"}}) {
		t.Fatal("participant shortcut must not bypass demo-mode hiding")
	}
}
