package app

import (
	"database/sql"
	"net/http"
	"strings"
)

func (a *App) scopes() ([]Option, error) {
	depts, err := a.options("md_department")
	if err != nil {
		return nil, err
	}
	out := []Option{{Code: "ALL", Name: "Toàn công ty"}}
	out = append(out, depts...)
	return out, nil
}

func (a *App) basePage(u *User, title, subtitle, active string) PageData {
	return PageData{User: u, Title: title, Subtitle: subtitle, Active: active}
}

func (a *App) dashboard(w http.ResponseWriter, r *http.Request, u *User) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	stats, err := a.dashboardStats(u)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	d := a.basePage(u, "Tổng quan điều hành", "Chinfon Kaizen 360° · PN-B0000-02 (2026)", "dashboard")
	d.Stats = stats
	applyFlash(&d, r)
	a.render(w, "dashboard", d)
}

func (a *App) initiativesList(w http.ResponseWriter, r *http.Request, u *User) {
	q := r.URL.Query()
	f := ListFilter{
		Track:  strings.TrimSpace(q.Get("track")),
		Stage:  strings.TrimSpace(q.Get("stage")),
		Status: strings.TrimSpace(q.Get("status")),
		Query:  strings.TrimSpace(q.Get("q")),
		Limit:  500,
	}
	if f.Status == "" {
		f.Status = StatusOpen
	}
	if f.Status == "ALL" {
		f.Status = ""
	}
	list, err := a.listInitiatives(u, f)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	tracks, _ := a.options("md_track")
	stages, _ := a.options("md_stage")
	d := a.basePage(u, "Hồ sơ sáng kiến", "Phân luồng 3-Track · phạm vi dữ liệu theo cem-auth", "initiatives")
	d.Initiatives = list
	d.Tracks = tracks
	d.Stages = stages
	d.Filters = map[string]string{
		"track":  q.Get("track"),
		"stage":  q.Get("stage"),
		"status": q.Get("status"),
		"q":      q.Get("q"),
	}
	applyFlash(&d, r)
	a.render(w, "initiatives", d)
}

func (a *App) loadInitiativeDetailData(u *User, id int64) (*PageData, int, error) {
	i, err := a.getInitiative(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, http.StatusNotFound, err
		}
		return nil, http.StatusInternalServerError, err
	}
	participants, err := a.loadParticipants(id)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	if !u.CanViewWithParticipants(i, participants) {
		if i.TrackCode == TrackConfidential {
			return nil, http.StatusNotFound, nil
		}
		return nil, http.StatusForbidden, nil
	}
	reviews, err := a.loadReviews(id)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	history, err := a.loadHistory(id)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	atts, err := a.loadAttachments(id)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	docTypes, _ := a.options("md_document_type")
	implStatuses, _ := a.options("md_implementation_status")

	d := a.basePage(u, i.Code+" — "+i.Title, i.DepartmentName, "initiatives")
	d.Initiative = i
	d.Participants = participants
	d.Reviews = reviews
	d.History = history
	d.Attachments = atts
	d.DocumentTypes = docTypes
	d.ImplementationStatuses = implStatuses
	d.CanEdit = i.StageCode == StageDraft && i.RecordStatus == StatusOpen &&
		(i.SubmitterEmployeeNo == u.EmployeeNo || u.IsAdmin())
	d.CanSubmit = d.CanEdit
	d.ActPerm = stagePerm(i.StageCode)
	d.ActLabel = stageActionLabel(i.StageCode)
	d.SelfReview = u.SelfReviewBlocked(i)
	d.CanAct = d.ActPerm != "" && u.CanAct(i, d.ActPerm) && !d.SelfReview
	d.DefaultReward = ComputeReward(i.NetBenefitVND, i.Annex03, i.RewardBasis, i.NonFinancialLevel)
	d.CanContribute = a.canContribute(u, i, participants)
	if d.CanEdit {
		emps, _ := a.masterEmployees()
		for _, e := range emps {
			if e.Active {
				d.Employees = append(d.Employees, e)
			}
		}
	}
	return &d, http.StatusOK, nil
}

func (a *App) initiativeDetail(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	d, status, err := a.loadInitiativeDetailData(u, id)
	if status == http.StatusNotFound {
		a.audit(r, u.EmployeeNo, "READ_DENIED", r.URL.Path, "404 DENIED", "Không tồn tại hoặc bị giới hạn bảo mật")
		http.NotFound(w, r)
		return
	}
	if status == http.StatusForbidden {
		a.forbidden(w, r, u, "READ_DENIED", r.URL.Path, "Ngoài phạm vi đơn vị hoặc thiếu quyền")
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if d.Initiative.TrackCode == TrackConfidential {
		a.audit(r, u.EmployeeNo, "READ_CONFIDENTIAL", d.Initiative.Code, "200 OK", "Giải mật Luồng 3")
	}
	applyFlash(d, r)
	a.render(w, "detail", *d)
}

func (a *App) initiativePrint(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	d, status, err := a.loadInitiativeDetailData(u, id)
	if status == http.StatusNotFound {
		http.NotFound(w, r)
		return
	}
	if status == http.StatusForbidden {
		a.forbidden(w, r, u, "PRINT_DENIED", r.URL.Path, "Ngoài phạm vi dữ liệu")
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if d.Initiative.TrackCode == TrackConfidential {
		a.audit(r, u.EmployeeNo, "PRINT_CONFIDENTIAL", d.Initiative.Code, "200 OK", "In phiếu thẩm định Luồng 3")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	a.render(w, "print", *d)
}

func (a *App) oplPage(w http.ResponseWriter, r *http.Request, u *User) {
	list, err := a.listInitiatives(u, ListFilter{OPL: true, Limit: 300})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	d := a.basePage(u, "Phụ lục 03 & Kho tri thức OPL", "Sáng kiến đã áp dụng thực tế, sẵn sàng nhân rộng", "opl")
	d.OPL = list
	applyFlash(&d, r)
	a.render(w, "opl", d)
}

func (a *App) rewardsPage(w http.ResponseWriter, r *http.Request, u *User) {
	if !u.HasPerm(PermPayroll) && !u.IsAdmin() {
		a.forbidden(w, r, u, "VIEW_PAYROLL", "/rewards", "Thiếu quyền hr:payroll:export")
		return
	}
	rows, totals, err := a.rewardLedger(u)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	tiers, err := a.rewardTiers()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	d := a.basePage(u, "Khung 7 mức thưởng & sổ cái HRD", "PN-B0000-02 Mục 5.1 / 5.2 · hệ số 1.2x Phụ lục 03", "rewards")
	for _, t := range tiers {
		if t.Basis == BasisNonFinancial {
			d.NonFinancial = append(d.NonFinancial, t)
		} else {
			d.RewardTiers = append(d.RewardTiers, t)
		}
	}
	d.RewardRows = rows
	d.RewardTotals = totals
	applyFlash(&d, r)
	a.audit(r, u.EmployeeNo, "VIEW_PAYROLL", "/rewards", "200 OK", "Xem sổ cái chi trả")
	a.render(w, "rewards", d)
}

func (a *App) rewardTiers() ([]RewardTier, error) {
	rows, err := a.DB.Query(`
		SELECT code,name,basis,min_net_vnd,max_net_vnd,base_amount_vnd,rule_note,sort_order
		FROM md_reward_tier ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RewardTier
	for rows.Next() {
		var t RewardTier
		if err := rows.Scan(&t.Code, &t.Name, &t.Basis, &t.MinNetVND, &t.MaxNetVND, &t.BaseAmountVND, &t.RuleNote, &t.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
