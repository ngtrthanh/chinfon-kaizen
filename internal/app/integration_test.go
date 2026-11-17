package app

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestApp(t *testing.T, seed bool) *App {
	t.Helper()
	dir := t.TempDir()
	a, err := New(Config{
		DBPath:    filepath.Join(dir, "test.db"),
		UploadDir: filepath.Join(dir, "uploads"),
		SeedDemo:  false,
		Logger:    log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	if seed {
		// Migration 007 turns demo mode off for production. Tests that need the
		// seed fixtures explicitly enable it before seeding.
		if _, err := a.DB.Exec(`UPDATE app_setting SET value='1' WHERE key='demo_mode'`); err != nil {
			t.Fatal(err)
		}
		if err := seedDemoData(a.DB); err != nil {
			t.Fatal(err)
		}
	}
	return a
}

func TestSeedVisibilityEnforcedInSQL(t *testing.T) {
	a := newTestApp(t, true)

	worker := testUser("CF-0412", "KILN", RoleWorker)
	list, err := a.listInitiatives(worker, ListFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].SubmitterEmployeeNo != "CF-0412" {
		t.Fatalf("worker should see exactly own record, got %d", len(list))
	}

	supervisor := testUser("CF-0118", "KILN", RoleSupervisor)
	list, err = a.listInitiatives(supervisor, ListFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("supervisor should see 2 KILN records, got %d", len(list))
	}
	for _, i := range list {
		if i.DepartmentCode != "KILN" || i.TrackCode == TrackConfidential {
			t.Fatalf("scope leak: %s %s", i.DepartmentCode, i.TrackCode)
		}
	}

	council := testUser("CF-0010", "ALL", RoleCouncil)
	list, err = a.listInitiatives(council, ListFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 7 {
		t.Fatalf("council should see all 7 standard/fast records, got %d", len(list))
	}
	for _, i := range list {
		if i.TrackCode == TrackConfidential {
			t.Fatal("confidential leak to council")
		}
	}

	ceo := testUser("CF-0001", "ALL", RoleCEOAudit)
	list, err = a.listInitiatives(ceo, ListFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 8 {
		t.Fatalf("CEO should see all 8 records, got %d", len(list))
	}

	hrd := testUser("CF-0088", "ALL", RoleHRD)
	list, err = a.listInitiatives(hrd, ListFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range list {
		if i.TrackCode == TrackConfidential {
			t.Fatal("HRD must not see confidential track")
		}
	}

	secretSubmitter := testUser("CF-0911", "LOGISTICS", RoleWorker)
	list, err = a.listInitiatives(secretSubmitter, ListFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("confidential submitter should see own report plus participant record, got %d", len(list))
	}
	foundSecret := false
	for _, i := range list {
		if i.TrackCode == TrackConfidential {
			foundSecret = true
			if i.SubmitterEmployeeNo != "CF-0911" {
				t.Fatal("confidential submitter can only see their own report")
			}
		}
	}
	if !foundSecret {
		t.Fatal("confidential submitter should see their own report")
	}
}

func TestSeededRewardMatchesPolicy(t *testing.T) {
	a := newTestApp(t, true)
	ceo := testUser("CF-0001", "ALL", RoleCEOAudit)
	list, err := a.listInitiatives(ceo, ListFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range list {
		want := ComputeFinancialReward(i.NetBenefitVND, i.Annex03)
		if i.RewardTierCode != want.TierCode || i.RewardAmountVND != want.AmountVND {
			t.Fatalf("%s reward mismatch: got %s/%d want %s/%d",
				i.Code, i.RewardTierCode, i.RewardAmountVND, want.TierCode, want.AmountVND)
		}
	}
}

func TestHandlerAuthGateAndSetup(t *testing.T) {
	a := newTestApp(t, true)
	h := a.Handler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/setup", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup should render, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Thiết lập lần đầu") {
		t.Fatal("setup page content missing")
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("unauthenticated / should redirect to login, got %d %q", rec.Code, rec.Header().Get("Location"))
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "default-src 'self'") {
		t.Fatalf("health/security headers failed: %d %q", rec.Code, rec.Header().Get("Content-Security-Policy"))
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/static/app.css", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
		t.Fatalf("embedded static asset not served: %d", rec.Code)
	}
}

func TestAuditAndCodeAreImmutable(t *testing.T) {
	a := newTestApp(t, true)
	if _, err := a.DB.Exec(`UPDATE audit_log SET detail='tampered' WHERE id=(SELECT MIN(id) FROM audit_log)`); err == nil {
		t.Fatal("audit_log update should be blocked by trigger")
	}
	if _, err := a.DB.Exec(`DELETE FROM audit_log WHERE id=(SELECT MIN(id) FROM audit_log)`); err == nil {
		t.Fatal("audit_log delete should be blocked by trigger")
	}
	if _, err := a.DB.Exec(`UPDATE initiative SET code='SI-1999-99999' WHERE id=1`); err == nil {
		t.Fatal("initiative code must be immutable")
	}
}

func TestMasterUsersDoesNotDeadlock(t *testing.T) {
	a := newTestApp(t, false)
	done := make(chan error, 1)
	go func() {
		_, err := a.masterUsers()
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("masterUsers deadlocked on a single SQLite connection")
	}
}
