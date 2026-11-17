package app

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

type ListFilter struct {
	Track  string
	Stage  string
	Status string
	Query  string
	OPL    bool
	Limit  int
}

type InitiativeInput struct {
	TrackCode                string
	Title                    string
	DepartmentCode           string
	EquipmentCode            string
	ProblemSummary           string
	ProblemDetail            string
	SolutionSummary          string
	SolutionDetail           string
	BenefitSummary           string
	ImplementationStatusCode string
	GrossValueVND            int64
	InvestCostVND            int64
	Annex03                  bool
	SplitRatio               string
}

func (a *App) options(table string) ([]Option, error) {
	allowed := map[string]string{
		"md_department":            `SELECT code,name FROM md_department WHERE active=1 AND is_demo=0 ORDER BY sort_order, name`,
		"md_track":                 `SELECT code,name FROM md_track ORDER BY sort_order`,
		"md_stage":                 `SELECT code,name FROM md_stage ORDER BY sort_order`,
		"md_implementation_status": `SELECT code,name FROM md_implementation_status ORDER BY sort_order`,
		"md_document_type":         `SELECT code,name FROM md_document_type ORDER BY code`,
		"md_reward_tier":           `SELECT code,name FROM md_reward_tier ORDER BY sort_order`,
		"rbac_role":                `SELECT code,name FROM rbac_role ORDER BY sort_order`,
		"rbac_permission":          `SELECT code,name FROM rbac_permission ORDER BY sort_order`,
	}
	q, ok := allowed[table]
	if !ok {
		return nil, fmt.Errorf("unsupported master table")
	}
	rows, err := a.DB.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Option
	for rows.Next() {
		var o Option
		if err := rows.Scan(&o.Code, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

const initiativeColumns = `
	i.id, i.code, i.track_code, t.name, i.stage_code, s.name, i.record_status, i.title,
	i.department_code, d.name, i.submitter_employee_no, e.full_name, i.equipment_code,
	i.problem_summary, i.problem_detail, i.solution_summary, i.solution_detail, i.benefit_summary,
	i.implementation_status_code, ims.name,
	i.gross_value_vnd, i.invest_cost_vnd, i.net_benefit_vnd, i.annex03,
	i.reward_basis, i.nonfinancial_level,
	COALESCE(i.reward_tier_code,''), COALESCE(rt.name,''),
	i.base_reward_vnd, i.reward_amount_vnd, i.seed_granted, i.anonymous, i.split_ratio,
	i.due_at, i.submitted_at, i.paid_at, i.paid_by, COALESCE(pb.full_name,''), i.reject_reason,
	i.created_by, i.created_at, i.updated_at, i.is_demo,
	(SELECT COUNT(*) FROM initiative_review r WHERE r.initiative_id=i.id),
	(SELECT COUNT(*) FROM initiative_attachment a WHERE a.initiative_id=i.id)
`

const initiativeFrom = `
	FROM initiative i
	JOIN md_track t ON t.code=i.track_code
	JOIN md_stage s ON s.code=i.stage_code
	JOIN md_department d ON d.code=i.department_code
	JOIN md_employee e ON e.employee_no=i.submitter_employee_no
	JOIN md_implementation_status ims ON ims.code=i.implementation_status_code
	LEFT JOIN md_reward_tier rt ON rt.code=i.reward_tier_code
	LEFT JOIN md_employee pb ON pb.employee_no=i.paid_by
`

func scanInitiative(s interface{ Scan(...any) error }) (Initiative, error) {
	var i Initiative
	var annex, seed, anon, isDemo int
	err := s.Scan(
		&i.ID, &i.Code, &i.TrackCode, &i.TrackName, &i.StageCode, &i.StageName, &i.RecordStatus, &i.Title,
		&i.DepartmentCode, &i.DepartmentName, &i.SubmitterEmployeeNo, &i.SubmitterName, &i.EquipmentCode,
		&i.ProblemSummary, &i.ProblemDetail, &i.SolutionSummary, &i.SolutionDetail, &i.BenefitSummary,
		&i.ImplementationStatusCode, &i.ImplementationStatusName,
		&i.GrossValueVND, &i.InvestCostVND, &i.NetBenefitVND, &annex,
		&i.RewardBasis, &i.NonFinancialLevel,
		&i.RewardTierCode, &i.RewardTierName,
		&i.BaseRewardVND, &i.RewardAmountVND, &seed, &anon, &i.SplitRatio,
		&i.DueAt, &i.SubmittedAt, &i.PaidAt, &i.PaidBy, &i.PaidByName, &i.RejectReason,
		&i.CreatedBy, &i.CreatedAt, &i.UpdatedAt, &isDemo,
		&i.ReviewCount, &i.AttachmentCount,
	)
	i.Annex03 = annex == 1
	i.SeedGranted = seed == 1
	i.Anonymous = anon == 1
	i.IsDemo = isDemo == 1
	return i, err
}

func (a *App) listInitiatives(u *User, f ListFilter) ([]Initiative, error) {
	if f.Limit <= 0 || f.Limit > 5000 {
		f.Limit = 500
	}
	where, args := u.VisibilityClause()
	var clauses []string
	if where != "" {
		clauses = append(clauses, where)
	}
	if f.Track != "" {
		clauses = append(clauses, "i.track_code = ?")
		args = append(args, f.Track)
	}
	if f.Stage != "" {
		clauses = append(clauses, "i.stage_code = ?")
		args = append(args, f.Stage)
	}
	if f.Status != "" {
		clauses = append(clauses, "i.record_status = ?")
		args = append(args, f.Status)
	}
	if f.OPL {
		clauses = append(clauses, "i.implementation_status_code = 'IMPLEMENTED' AND i.stage_code IN ('IMPLEMENT','APPLIED')")
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		clauses = append(clauses, `(i.code LIKE ? OR i.title LIKE ? OR e.full_name LIKE ? OR i.equipment_code LIKE ?)`)
		like := "%" + q + "%"
		args = append(args, like, like, like, like)
	}
	q := "SELECT " + initiativeColumns + initiativeFrom
	if len(clauses) > 0 {
		q += " WHERE " + strings.Join(clauses, " AND ")
	}
	q += " ORDER BY i.id DESC LIMIT ?"
	args = append(args, f.Limit)

	rows, err := a.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Initiative
	for rows.Next() {
		i, err := scanInitiative(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (a *App) getInitiative(id int64) (*Initiative, error) {
	row := a.DB.QueryRow("SELECT "+initiativeColumns+initiativeFrom+" WHERE i.id=?", id)
	i, err := scanInitiative(row)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (a *App) loadParticipants(id int64) ([]Participant, error) {
	rows, err := a.DB.Query(`
		SELECT p.employee_no, e.full_name, d.name, p.share_percent, p.role_note
		FROM initiative_participant p
		JOIN md_employee e ON e.employee_no=p.employee_no
		JOIN md_department d ON d.code=e.department_code
		WHERE p.initiative_id=? ORDER BY p.share_percent DESC, e.full_name`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Participant
	for rows.Next() {
		var p Participant
		if err := rows.Scan(&p.EmployeeNo, &p.FullName, &p.DepartmentName, &p.SharePercent, &p.RoleNote); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (a *App) loadReviews(id int64) ([]Review, error) {
	rows, err := a.DB.Query(`
		SELECT r.id, r.stage_code, s.name, e.full_name, r.decision, r.comment,
		       r.assessed_gross_vnd, r.assessed_invest_vnd, r.assessed_net_vnd,
		       r.reward_tier_code, COALESCE(rt.name,''), r.reward_amount_vnd, r.created_at
		FROM initiative_review r
		JOIN md_stage s ON s.code=r.stage_code
		JOIN md_employee e ON e.employee_no=r.reviewer_employee_no
		LEFT JOIN md_reward_tier rt ON rt.code=r.reward_tier_code
		WHERE r.initiative_id=? ORDER BY r.id DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Review
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.StageCode, &r.StageName, &r.ReviewerName, &r.Decision, &r.Comment,
			&r.AssessedGrossVND, &r.AssessedInvestVND, &r.AssessedNetVND,
			&r.RewardTierCode, &r.RewardTierName, &r.RewardAmountVND, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (a *App) loadHistory(id int64) ([]HistoryItem, error) {
	rows, err := a.DB.Query(`
		SELECT h.id, COALESCE(h.from_stage_code,''), h.to_stage_code,
		       COALESCE(fs.name,''), ts.name, h.action_code, h.changed_by, e.full_name, h.note, h.created_at
		FROM initiative_history h
		JOIN md_stage ts ON ts.code=h.to_stage_code
		LEFT JOIN md_stage fs ON fs.code=h.from_stage_code
		JOIN md_employee e ON e.employee_no=h.changed_by
		WHERE h.initiative_id=? ORDER BY h.id DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HistoryItem
	for rows.Next() {
		var h HistoryItem
		if err := rows.Scan(&h.ID, &h.FromStage, &h.ToStage, &h.FromName, &h.ToName, &h.ActionCode,
			&h.ChangedBy, &h.ChangedName, &h.Note, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (a *App) loadAttachments(id int64) ([]Attachment, error) {
	rows, err := a.DB.Query(`
		SELECT a.id, a.document_type_code, dt.name, a.original_filename, a.storage_key,
		       COALESCE(a.mime_type,''), a.size_bytes, a.sha256_hex, a.uploaded_by, e.full_name, a.uploaded_at
		FROM initiative_attachment a
		JOIN md_document_type dt ON dt.code=a.document_type_code
		JOIN md_employee e ON e.employee_no=a.uploaded_by
		WHERE a.initiative_id=? ORDER BY a.id DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Attachment
	for rows.Next() {
		var x Attachment
		if err := rows.Scan(&x.ID, &x.DocumentTypeCode, &x.DocumentTypeName, &x.OriginalFilename, &x.StorageKey,
			&x.MimeType, &x.SizeBytes, &x.SHA256Hex, &x.UploadedBy, &x.UploadedByName, &x.UploadedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (a *App) dashboardStats(u *User) (*DashboardStats, error) {
	all, err := a.listInitiatives(u, ListFilter{Limit: 5000})
	if err != nil {
		return nil, err
	}
	st := &DashboardStats{
		Stages: map[string]int{
			StageDraft: 0, StageScreen: 0, StageReview: 0, StageApprove: 0, StageImplement: 0, StageApplied: 0,
		},
		Tracks: map[string]int{TrackFast48: 0, TrackStandard: 0, TrackConfidential: 0},
	}
	now := time.Now()
	areaMap := map[string]*AreaStat{}
	for _, i := range all {
		st.Total++
		if i.RecordStatus == StatusOpen {
			st.Open++
		}
		if i.RecordStatus != StatusRejected {
			st.NetVND += i.NetBenefitVND
			if i.StageCode == StageImplement || i.StageCode == StageApplied {
				st.RewardApproved += i.RewardAmountVND
			}
			if i.PaidAt.Valid {
				st.RewardPaid += i.RewardAmountVND
			}
		}
		if i.TrackCode == TrackConfidential {
			st.Confidential++
		}
		if i.TrackCode == TrackFast48 {
			st.Fast48++
		}
		if i.ImplementationStatusCode == "IDEA" {
			st.Ideas++
		}
		if i.StageCode == StageApplied {
			st.Applied++
		}
		st.Stages[i.StageCode]++
		st.Tracks[i.TrackCode]++
		if i.RecordStatus == StatusOpen && i.StageCode != StageApplied && i.StageCode != StageDraft && i.DueAt.Valid {
			if due, ok := parseAppTime(i.DueAt.String); ok && due.Before(now) {
				st.Overdue++
				if len(st.OverdueList) < 5 {
					st.OverdueList = append(st.OverdueList, i)
				}
			}
		}
		area := areaMap[i.DepartmentCode]
		if area == nil {
			area = &AreaStat{Code: i.DepartmentCode, Name: i.DepartmentName}
			areaMap[i.DepartmentCode] = area
		}
		if i.RecordStatus != StatusRejected {
			area.NetVND += i.NetBenefitVND
			area.Count++
		}
		if len(st.Recent) < 6 {
			st.Recent = append(st.Recent, i)
		}
	}
	for _, area := range areaMap {
		st.Areas = append(st.Areas, *area)
	}
	sort.Slice(st.Areas, func(i, j int) bool { return st.Areas[i].NetVND > st.Areas[j].NetVND })
	var max int64
	for _, area := range st.Areas {
		if area.NetVND > max {
			max = area.NetVND
		}
	}
	for idx := range st.Areas {
		if max > 0 {
			st.Areas[idx].Pct = int(float64(st.Areas[idx].NetVND) / float64(max) * 100)
		}
	}
	st.TopUnits = append(st.TopUnits, st.Areas...)
	if len(st.TopUnits) > 4 {
		st.TopUnits = st.TopUnits[:4]
	}
	return st, nil
}

func (a *App) rewardLedger(u *User) ([]RewardRow, RewardTotals, error) {
	all, err := a.listInitiatives(u, ListFilter{Limit: 5000})
	if err != nil {
		return nil, RewardTotals{}, err
	}
	var rows []RewardRow
	var totals RewardTotals
	for _, i := range all {
		if i.RewardAmountVND <= 0 {
			continue
		}
		if i.StageCode != StageImplement && i.StageCode != StageApplied {
			continue
		}
		if i.RecordStatus == StatusRejected {
			continue
		}
		participants, err := a.loadParticipants(i.ID)
		if err != nil {
			return nil, totals, err
		}
		shares := map[string]float64{i.SubmitterEmployeeNo: 100}
		names := map[string]string{i.SubmitterEmployeeNo: i.SubmitterName}
		for _, p := range participants {
			shares[p.EmployeeNo] = p.SharePercent
			names[p.EmployeeNo] = p.FullName
		}
		remaining := 100.0
		for _, p := range participants {
			remaining -= p.SharePercent
		}
		if remaining < 0 {
			remaining = 0
		}
		shares[i.SubmitterEmployeeNo] = remaining
		for empNo, share := range shares {
			if share <= 0 {
				continue
			}
			amount := int64(float64(i.RewardAmountVND) * share / 100.0)
			rows = append(rows, RewardRow{
				InitiativeID:   i.ID,
				Code:           i.Code,
				TrackCode:      i.TrackCode,
				DepartmentName: i.DepartmentName,
				EmployeeNo:     empNo,
				EmployeeName:   names[empNo],
				SharePercent:   share,
				AmountVND:      amount,
				TierName:       i.RewardTierName,
				PaidAt:         i.PaidAt,
				StageCode:      i.StageCode,
			})
			totals.Headcount++
		}
		totals.ApprovedVND += i.RewardAmountVND
		if i.PaidAt.Valid {
			totals.PaidVND += i.RewardAmountVND
		}
	}
	totals.PendingVND = totals.ApprovedVND - totals.PaidVND
	sort.Slice(rows, func(x, y int) bool { return rows[x].InitiativeID > rows[y].InitiativeID })
	return rows, totals, nil
}

func (a *App) nextInitiativeCode(tx *sql.Tx, year int) (string, error) {
	var n int
	err := tx.QueryRow(`
		INSERT INTO initiative_sequence(year,value) VALUES(?,1)
		ON CONFLICT(year) DO UPDATE SET value = value + 1
		RETURNING value`, year).Scan(&n)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("SI-%04d-%05d", year, n), nil
}

func (a *App) createInitiative(u *User, in InitiativeInput) (*Initiative, error) {
	net := NetBenefit(in.GrossValueVND, in.InvestCostVND)
	reward := ComputeFinancialReward(net, in.Annex03)
	anon := 0
	if in.TrackCode == TrackConfidential {
		anon = 1
	}
	annex := 0
	if in.Annex03 {
		annex = 1
	}
	tx, err := a.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	code, err := a.nextInitiativeCode(tx, time.Now().Year())
	if err != nil {
		return nil, err
	}
	res, err := tx.Exec(`
		INSERT INTO initiative(code,track_code,stage_code,record_status,title,department_code,submitter_employee_no,
			equipment_code,problem_summary,problem_detail,solution_summary,solution_detail,benefit_summary,
			implementation_status_code,gross_value_vnd,invest_cost_vnd,net_benefit_vnd,annex03,
			reward_basis,nonfinancial_level,reward_tier_code,base_reward_vnd,reward_amount_vnd,
			anonymous,split_ratio,created_by)
		VALUES(?,?,?,'OPEN',?,?,?,?,?,?,?,?,?,?,?,?,?,?,'FINANCIAL',0,?,?,?,?,?,?)`,
		code, in.TrackCode, StageDraft, in.Title, in.DepartmentCode, u.EmployeeNo,
		in.EquipmentCode, in.ProblemSummary, in.ProblemDetail, in.SolutionSummary, in.SolutionDetail, in.BenefitSummary,
		in.ImplementationStatusCode, in.GrossValueVND, in.InvestCostVND, net, annex,
		reward.TierCode, reward.BaseVND, reward.AmountVND, anon, in.SplitRatio, u.EmployeeNo)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`INSERT INTO initiative_history(initiative_id,to_stage_code,action_code,changed_by,note)
		VALUES(?,?,'CREATED',?,'Khởi tạo hồ sơ')`, id, StageDraft, u.EmployeeNo); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return a.getInitiative(id)
}

func (a *App) updateInitiative(i *Initiative, in InitiativeInput) error {
	net := NetBenefit(in.GrossValueVND, in.InvestCostVND)
	reward := ComputeFinancialReward(net, in.Annex03)
	annex := 0
	if in.Annex03 {
		annex = 1
	}
	anon := 0
	if in.TrackCode == TrackConfidential {
		anon = 1
	}
	_, err := a.DB.Exec(`
		UPDATE initiative SET title=?, track_code=?, department_code=?, equipment_code=?,
			problem_summary=?, problem_detail=?, solution_summary=?, solution_detail=?, benefit_summary=?,
			implementation_status_code=?, gross_value_vnd=?, invest_cost_vnd=?, net_benefit_vnd=?,
			annex03=?, reward_tier_code=?, base_reward_vnd=?, reward_amount_vnd=?, anonymous=?,
			split_ratio=?, updated_at=?
		WHERE id=? AND stage_code=? AND record_status='OPEN'`,
		in.Title, in.TrackCode, in.DepartmentCode, in.EquipmentCode,
		in.ProblemSummary, in.ProblemDetail, in.SolutionSummary, in.SolutionDetail, in.BenefitSummary,
		in.ImplementationStatusCode, in.GrossValueVND, in.InvestCostVND, net,
		annex, reward.TierCode, reward.BaseVND, reward.AmountVND, anon,
		in.SplitRatio, nowUTC(), i.ID, StageDraft)
	return err
}

func (a *App) submitInitiative(i *Initiative, u *User) error {
	to := StageScreen
	if i.TrackCode == TrackConfidential {
		to = StageReview
	}
	now := time.Now().UTC()
	due := dueForTrack(i.TrackCode, now)
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`
		UPDATE initiative SET stage_code=?, submitted_at=?, due_at=?, updated_at=?
		WHERE id=? AND stage_code=? AND record_status='OPEN'`,
		to, now.Format(time.RFC3339), due.Format(time.RFC3339), now.Format(time.RFC3339), i.ID, StageDraft)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return fmt.Errorf("hồ sơ đã được xử lý bởi thao tác khác")
	}
	if _, err := tx.Exec(`INSERT INTO initiative_history(initiative_id,from_stage_code,to_stage_code,action_code,changed_by,note)
		VALUES(?,?,?,'SUBMITTED',?,?)`, i.ID, StageDraft, to, u.EmployeeNo, noteForTrack(i.TrackCode)); err != nil {
		return err
	}
	return tx.Commit()
}

func noteForTrack(track string) string {
	switch track {
	case TrackFast48:
		return "Nộp Luồng 1 — SLA 48 giờ"
	case TrackConfidential:
		return "Nộp Luồng 3 — chuyển thẳng TGĐ & Kiểm toán Nội bộ"
	default:
		return "Nộp Luồng 2 — SLA 5 ngày làm việc"
	}
}

func (a *App) addParticipant(i *Initiative, employeeNo string, share float64, roleNote string) error {
	if i.StageCode != StageDraft || i.RecordStatus != StatusOpen {
		return fmt.Errorf("chỉ khai báo tỷ lệ chia thưởng khi hồ sơ còn nháp")
	}
	if employeeNo == i.SubmitterEmployeeNo {
		return fmt.Errorf("người nộp hồ sơ đã mặc định nhận phần còn lại")
	}
	var exists int
	if err := a.DB.QueryRow(`SELECT COUNT(*) FROM md_employee WHERE employee_no=? AND active=1`, employeeNo).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return fmt.Errorf("không tìm thấy nhân viên")
	}
	var sum float64
	if err := a.DB.QueryRow(`SELECT COALESCE(SUM(share_percent),0) FROM initiative_participant WHERE initiative_id=? AND employee_no<>?`,
		i.ID, employeeNo).Scan(&sum); err != nil {
		return err
	}
	if sum+share > 100.0001 {
		return fmt.Errorf("tổng tỷ lệ đồng tác giả không được vượt quá 100%%")
	}
	_, err := a.DB.Exec(`INSERT INTO initiative_participant(initiative_id,employee_no,share_percent,role_note)
		VALUES(?,?,?,?)
		ON CONFLICT(initiative_id,employee_no) DO UPDATE SET share_percent=excluded.share_percent, role_note=excluded.role_note`,
		i.ID, employeeNo, share, roleNote)
	return err
}

func (a *App) removeParticipant(i *Initiative, employeeNo string) error {
	if i.StageCode != StageDraft || i.RecordStatus != StatusOpen {
		return fmt.Errorf("chỉ sửa tỷ lệ chia thưởng khi hồ sơ còn nháp")
	}
	_, err := a.DB.Exec(`DELETE FROM initiative_participant WHERE initiative_id=? AND employee_no=?`, i.ID, employeeNo)
	return err
}

// ---------------------------------------------------------------------------
// Master data, users, RBAC and audit
// ---------------------------------------------------------------------------

func (a *App) masterDepartments() ([]Department, error) {
	rows, err := a.DB.Query(`SELECT code,name,kind,sort_order,active FROM md_department WHERE is_demo=0 ORDER BY sort_order, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Department
	for rows.Next() {
		var d Department
		var active int
		if err := rows.Scan(&d.Code, &d.Name, &d.Kind, &d.SortOrder, &active); err != nil {
			return nil, err
		}
		d.Active = active == 1
		out = append(out, d)
	}
	return out, rows.Err()
}

func (a *App) masterEmployees() ([]Employee, error) {
	rows, err := a.DB.Query(`
		SELECT e.employee_no,e.full_name,e.department_code,d.name,COALESCE(e.position_title,''),COALESCE(e.work_group,''),e.active
		FROM md_employee e JOIN md_department d ON d.code=e.department_code
		WHERE e.is_demo=0
		ORDER BY d.sort_order, e.full_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Employee
	for rows.Next() {
		var e Employee
		var active int
		if err := rows.Scan(&e.EmployeeNo, &e.FullName, &e.DepartmentCode, &e.DepartmentName, &e.PositionTitle, &e.WorkGroup, &active); err != nil {
			return nil, err
		}
		e.Active = active == 1
		out = append(out, e)
	}
	return out, rows.Err()
}

func (a *App) masterUsers() ([]UserRow, error) {
	deptNames := map[string]string{}
	deptRows, err := a.DB.Query(`SELECT code,name FROM md_department`)
	if err == nil {
		for deptRows.Next() {
			var code, name string
			_ = deptRows.Scan(&code, &name)
			deptNames[code] = name
		}
		deptRows.Close()
	}
	rows, err := a.DB.Query(`
		SELECT u.id,u.username,u.employee_no,e.full_name,u.dept_scope,u.active,u.created_at,
		       COALESCE(u.totp_confirmed,0),
		       COALESCE(GROUP_CONCAT(ur.role_code, ','),'')
		FROM app_user u
		JOIN md_employee e ON e.employee_no=u.employee_no
		LEFT JOIN app_user_role ur ON ur.user_id=u.id
		GROUP BY u.id,u.username,u.employee_no,e.full_name,u.dept_scope,u.active,u.created_at,u.totp_confirmed
		ORDER BY u.username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserRow
	for rows.Next() {
		var x UserRow
		var active, must int
		var roles string
		if err := rows.Scan(&x.ID, &x.Username, &x.EmployeeNo, &x.FullName, &x.Scope, &active, &x.CreatedAt, &must, &roles); err != nil {
			return nil, err
		}
		x.Active = active == 1
		x.MFAEnrolled = must == 1
		if roles != "" {
			x.Roles = strings.Split(roles, ",")
			x.RoleNames = strings.Join(x.Roles, ", ")
		}
		if strings.EqualFold(x.Scope, "ALL") || x.Scope == "" {
			x.ScopeName = "Toàn công ty"
		} else {
			x.ScopeName = deptNames[x.Scope]
			if x.ScopeName == "" {
				x.ScopeName = x.Scope
			}
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (a *App) roles() ([]Role, error) {
	rows, err := a.DB.Query(`SELECT code,name,description FROM rbac_role ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Role
	for rows.Next() {
		var r Role
		if err := rows.Scan(&r.Code, &r.Name, &r.Description); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (a *App) permissions() ([]Permission, error) {
	rows, err := a.DB.Query(`SELECT code,name,description FROM rbac_permission ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Permission
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.Code, &p.Name, &p.Description); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (a *App) permissionMatrix() (map[string]map[string]bool, error) {
	rows, err := a.DB.Query(`SELECT role_code,permission_code FROM rbac_role_permission`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]map[string]bool{}
	for rows.Next() {
		var role, perm string
		if err := rows.Scan(&role, &perm); err != nil {
			return nil, err
		}
		if m[role] == nil {
			m[role] = map[string]bool{}
		}
		m[role][perm] = true
	}
	return m, rows.Err()
}

func (a *App) setRolePermission(role, perm string, allow bool) error {
	if allow {
		_, err := a.DB.Exec(`INSERT OR IGNORE INTO rbac_role_permission(role_code,permission_code) VALUES(?,?)`, role, perm)
		return err
	}
	_, err := a.DB.Exec(`DELETE FROM rbac_role_permission WHERE role_code=? AND permission_code=?`, role, perm)
	return err
}

func (a *App) auditList(q string, limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	query := `SELECT id,at,actor,action_code,target,status,detail,ip FROM audit_log`
	var args []any
	if q = strings.TrimSpace(q); q != "" {
		query += ` WHERE actor LIKE ? OR action_code LIKE ? OR target LIKE ? OR detail LIKE ?`
		like := "%" + q + "%"
		args = append(args, like, like, like, like)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := a.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var x AuditEntry
		if err := rows.Scan(&x.ID, &x.At, &x.Actor, &x.Action, &x.Target, &x.Status, &x.Detail, &x.IP); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (a *App) createUser(username, hash, employeeNo, scope string, roles []string) (int64, error) {
	tx, err := a.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	mustToken, err := randomToken(24)
	if err != nil {
		return 0, err
	}
	res, err := tx.Exec(`INSERT INTO app_user(username,password_hash,employee_no,dept_scope,active,must_change_password,csrf_token) VALUES(?,?,?,?,1,1,?)`,
		username, hash, employeeNo, scope, mustToken)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	for _, role := range roles {
		if _, err := tx.Exec(`INSERT INTO app_user_role(user_id,role_code) VALUES(?,?)`, id, role); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

func (a *App) updateUser(id int64, scope string, roles []string, active bool) error {
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	aFlag := 0
	if active {
		aFlag = 1
	}
	if _, err := tx.Exec(`UPDATE app_user SET dept_scope=?,active=? WHERE id=?`, scope, aFlag, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM app_user_role WHERE user_id=?`, id); err != nil {
		return err
	}
	for _, role := range roles {
		if _, err := tx.Exec(`INSERT INTO app_user_role(user_id,role_code) VALUES(?,?)`, id, role); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (a *App) countAdmins() (int, error) {
	var n int
	err := a.DB.QueryRow(`SELECT COUNT(DISTINCT u.id) FROM app_user u
		JOIN app_user_role ur ON ur.user_id=u.id
		WHERE ur.role_code='ADMIN' AND u.active=1`).Scan(&n)
	return n, err
}
