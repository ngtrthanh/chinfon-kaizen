package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var allowedUploadExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".pdf": true, ".csv": true, ".xlsx": true,
	".docx": true, ".txt": true,
}

func (a *App) validCode(table, code string) bool {
	allowed := map[string]string{
		"md_track":                 "md_track",
		"md_department":            "md_department",
		"md_implementation_status": "md_implementation_status",
		"md_document_type":         "md_document_type",
		"rbac_role":                "rbac_role",
		"rbac_permission":          "rbac_permission",
	}
	t, ok := allowed[table]
	if !ok {
		return false
	}
	var n int
	err := a.DB.QueryRow(`SELECT COUNT(*) FROM `+t+` WHERE code=?`, code).Scan(&n)
	return err == nil && n > 0
}

func (a *App) departmentActive(code string) bool {
	var n int
	err := a.DB.QueryRow(`SELECT COUNT(*) FROM md_department WHERE code=? AND active=1 AND is_demo=0`, code).Scan(&n)
	return err == nil && n > 0
}

func initiativeFormMap(r *http.Request) map[string]string {
	return map[string]string{
		"title":                 strings.TrimSpace(r.FormValue("title")),
		"track":                 strings.TrimSpace(r.FormValue("track")),
		"department":            strings.TrimSpace(r.FormValue("department")),
		"equipment":             strings.TrimSpace(r.FormValue("equipment")),
		"problem_summary":       strings.TrimSpace(r.FormValue("problem_summary")),
		"problem_detail":        strings.TrimSpace(r.FormValue("problem_detail")),
		"solution_summary":      strings.TrimSpace(r.FormValue("solution_summary")),
		"solution_detail":       strings.TrimSpace(r.FormValue("solution_detail")),
		"benefit_summary":       strings.TrimSpace(r.FormValue("benefit_summary")),
		"implementation_status": strings.TrimSpace(r.FormValue("implementation_status")),
		"gross_value":           strings.TrimSpace(r.FormValue("gross_value")),
		"invest_cost":           strings.TrimSpace(r.FormValue("invest_cost")),
		"annex03":               r.FormValue("annex03"),
		"split_ratio":           strings.TrimSpace(r.FormValue("split_ratio")),
	}
}

func mapFromInitiative(i *Initiative) map[string]string {
	annex := ""
	if i.Annex03 {
		annex = "1"
	}
	return map[string]string{
		"title":                 i.Title,
		"track":                 i.TrackCode,
		"department":            i.DepartmentCode,
		"equipment":             i.EquipmentCode,
		"problem_summary":       i.ProblemSummary,
		"problem_detail":        i.ProblemDetail,
		"solution_summary":      i.SolutionSummary,
		"solution_detail":       i.SolutionDetail,
		"benefit_summary":       i.BenefitSummary,
		"implementation_status": i.ImplementationStatusCode,
		"gross_value":           fmt.Sprintf("%d", i.GrossValueVND),
		"invest_cost":           fmt.Sprintf("%d", i.InvestCostVND),
		"annex03":               annex,
		"split_ratio":           i.SplitRatio,
	}
}

func (a *App) parseInitiativeInput(u *User, r *http.Request) (InitiativeInput, error) {
	var in InitiativeInput
	if err := r.ParseForm(); err != nil {
		return in, fmt.Errorf("không đọc được biểu mẫu")
	}
	in.TrackCode = strings.TrimSpace(r.FormValue("track"))
	in.Title = strings.TrimSpace(r.FormValue("title"))
	in.DepartmentCode = strings.TrimSpace(r.FormValue("department"))
	in.EquipmentCode = strings.TrimSpace(r.FormValue("equipment"))
	in.ProblemSummary = strings.TrimSpace(r.FormValue("problem_summary"))
	in.ProblemDetail = strings.TrimSpace(r.FormValue("problem_detail"))
	in.SolutionSummary = strings.TrimSpace(r.FormValue("solution_summary"))
	in.SolutionDetail = strings.TrimSpace(r.FormValue("solution_detail"))
	in.BenefitSummary = strings.TrimSpace(r.FormValue("benefit_summary"))
	in.ImplementationStatusCode = strings.TrimSpace(r.FormValue("implementation_status"))
	in.SplitRatio = strings.TrimSpace(r.FormValue("split_ratio"))
	in.Annex03 = r.FormValue("annex03") == "1" || r.FormValue("annex03") == "on"

	if !u.ScopeAll() {
		in.DepartmentCode = u.DepartmentCode
	}
	if in.DepartmentCode == "" {
		in.DepartmentCode = u.DepartmentCode
	}
	if !a.validCode("md_track", in.TrackCode) {
		return in, fmt.Errorf("luồng xử lý không hợp lệ")
	}
	if !a.departmentActive(in.DepartmentCode) {
		return in, fmt.Errorf("đơn vị không hợp lệ hoặc đã ngừng hoạt động")
	}
	if !a.validCode("md_implementation_status", in.ImplementationStatusCode) {
		return in, fmt.Errorf("trạng thái triển khai không hợp lệ")
	}
	if l := len([]rune(in.Title)); l < 5 || l > 240 {
		return in, fmt.Errorf("tiêu đề phải từ 5 đến 240 ký tự")
	}
	if in.ProblemSummary == "" || len([]rune(in.ProblemSummary)) > 500 {
		return in, fmt.Errorf("tóm tắt hiện trạng bắt buộc, tối đa 500 ký tự")
	}
	if strings.TrimSpace(in.ProblemDetail) == "" {
		return in, fmt.Errorf("mô tả hiện trạng bắt buộc")
	}
	if in.SolutionSummary == "" || len([]rune(in.SolutionSummary)) > 500 {
		return in, fmt.Errorf("tóm tắt giải pháp bắt buộc, tối đa 500 ký tự")
	}
	if strings.TrimSpace(in.SolutionDetail) == "" {
		return in, fmt.Errorf("mô tả giải pháp bắt buộc")
	}
	if strings.TrimSpace(in.BenefitSummary) == "" {
		return in, fmt.Errorf("tóm tắt lợi ích bắt buộc (Phụ lục 01)")
	}
	if len([]rune(in.BenefitSummary)) > 1000 || len([]rune(in.EquipmentCode)) > 120 || len([]rune(in.SplitRatio)) > 240 {
		return in, fmt.Errorf("một số trường vượt quá độ dài cho phép")
	}
	gross, hasGross, err := parseVND(r.FormValue("gross_value"))
	if err != nil {
		return in, fmt.Errorf("giá trị tiết kiệm 12 tháng không hợp lệ")
	}
	if !hasGross {
		gross = 0
	}
	invest, hasInvest, err := parseVND(r.FormValue("invest_cost"))
	if err != nil {
		return in, fmt.Errorf("chi phí đầu tư không hợp lệ")
	}
	if !hasInvest {
		invest = 0
	}
	in.GrossValueVND = gross
	in.InvestCostVND = invest
	return in, nil
}

func (a *App) fillInitiativeOptions(d *PageData) {
	d.Tracks, _ = a.options("md_track")
	d.Departments, _ = a.options("md_department")
	d.ImplementationStatuses, _ = a.options("md_implementation_status")
}

func (a *App) newInitiativePage(w http.ResponseWriter, r *http.Request, u *User, form map[string]string, errMsg string) {
	if !u.HasPerm(PermCreate) {
		a.forbidden(w, r, u, "CREATE_DENIED", "/initiatives/new", "Thiếu quyền idea:create")
		return
	}
	d := a.basePage(u, "Nộp Phụ lục 01", "Khai báo 30 giây · phân luồng tự động theo PN-B0000-02", "new")
	a.fillInitiativeOptions(&d)
	if form == nil {
		form = map[string]string{
			"track":                 TrackFast48,
			"department":            u.DepartmentCode,
			"implementation_status": "IDEA",
			"split_ratio":           "100% Tác giả",
			"annex03":               "1",
		}
		if v := strings.TrimSpace(r.URL.Query().Get("track")); v != "" {
			form["track"] = v
		}
	}
	d.Form = form
	d.Error = errMsg
	applyFlash(&d, r)
	a.render(w, "form", d)
}

func (a *App) initiativeNewGet(w http.ResponseWriter, r *http.Request, u *User) {
	a.newInitiativePage(w, r, u, nil, "")
}

func (a *App) initiativeCreatePost(w http.ResponseWriter, r *http.Request, u *User) {
	if !u.HasPerm(PermCreate) {
		a.forbidden(w, r, u, "CREATE_DENIED", "/initiatives", "Thiếu quyền idea:create")
		return
	}
	form := initiativeFormMap(r)
	in, err := a.parseInitiativeInput(u, r)
	if err != nil {
		a.newInitiativePage(w, r, u, form, err.Error())
		return
	}
	i, err := a.createInitiative(u, in)
	if err != nil {
		a.newInitiativePage(w, r, u, form, "Không tạo được hồ sơ: "+err.Error())
		return
	}
	detail := "Tạo hồ sơ " + i.TrackName
	if i.TrackCode == TrackConfidential {
		detail = "Tạo hồ sơ Luồng 3 bảo mật"
	}
	a.audit(r, u.EmployeeNo, "CREATE_INITIATIVE", i.Code, "201 CREATED", detail)
	if r.FormValue("action") == "submit" {
		if err := a.submitInitiative(i, u); err != nil {
			flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", i.ID), "Đã lưu nháp nhưng chưa nộp được: "+err.Error())
			return
		}
		a.audit(r, u.EmployeeNo, "SUBMIT_INITIATIVE", i.Code, "200 OK", noteForTrack(i.TrackCode))
		flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", i.ID), "Đã nộp hồ sơ. Hệ thống đã áp SLA theo luồng.")
		return
	}
	flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", i.ID), "Đã lưu nháp. Bạn có thể bổ sung đồng tác giả hoặc tài liệu trước khi nộp.")
}

func (a *App) initiativeEditGet(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	i, err := a.getInitiative(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if i.StageCode != StageDraft || i.RecordStatus != StatusOpen || !(i.SubmitterEmployeeNo == u.EmployeeNo || u.IsAdmin()) {
		a.forbidden(w, r, u, "EDIT_DENIED", i.Code, "Chỉ sửa hồ sơ nháp của chính mình")
		return
	}
	participants, _ := a.loadParticipants(id)
	d := a.basePage(u, "Sửa "+i.Code, i.Title, "initiatives")
	a.fillInitiativeOptions(&d)
	d.Initiative = i
	d.Participants = participants
	d.Form = mapFromInitiative(i)
	d.CanEdit = true
	emps, _ := a.masterEmployees()
	for _, e := range emps {
		if e.Active {
			d.Employees = append(d.Employees, e)
		}
	}
	applyFlash(&d, r)
	a.render(w, "form", d)
}

func (a *App) initiativeEditPost(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	i, err := a.getInitiative(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if i.StageCode != StageDraft || i.RecordStatus != StatusOpen || !(i.SubmitterEmployeeNo == u.EmployeeNo || u.IsAdmin()) {
		a.forbidden(w, r, u, "EDIT_DENIED", i.Code, "Chỉ sửa hồ sơ nháp của chính mình")
		return
	}
	form := initiativeFormMap(r)
	in, err := a.parseInitiativeInput(u, r)
	if err != nil {
		participants, _ := a.loadParticipants(id)
		d := a.basePage(u, "Sửa "+i.Code, i.Title, "initiatives")
		a.fillInitiativeOptions(&d)
		d.Initiative = i
		d.Participants = participants
		d.Form = form
		d.CanEdit = true
		d.Error = err.Error()
		emps, _ := a.masterEmployees()
		for _, e := range emps {
			if e.Active {
				d.Employees = append(d.Employees, e)
			}
		}
		a.render(w, "form", d)
		return
	}
	if err := a.updateInitiative(i, in); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	a.audit(r, u.EmployeeNo, "UPDATE_INITIATIVE", i.Code, "200 OK", "Cập nhật hồ sơ nháp")
	flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "Đã cập nhật hồ sơ nháp.")
}

func (a *App) initiativeSubmitPost(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	i, err := a.getInitiative(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if i.StageCode != StageDraft || i.RecordStatus != StatusOpen || !(i.SubmitterEmployeeNo == u.EmployeeNo || u.IsAdmin()) {
		a.forbidden(w, r, u, "SUBMIT_DENIED", i.Code, "Chỉ nộp hồ sơ nháp của chính mình")
		return
	}
	if err := a.submitInitiative(i, u); err != nil {
		flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), err.Error())
		return
	}
	a.audit(r, u.EmployeeNo, "SUBMIT_INITIATIVE", i.Code, "200 OK", noteForTrack(i.TrackCode))
	flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "Đã nộp hồ sơ. SLA đã được áp theo luồng.")
}

func (a *App) participantPost(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	i, err := a.getInitiative(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if i.StageCode != StageDraft || i.RecordStatus != StatusOpen || !(i.SubmitterEmployeeNo == u.EmployeeNo || u.IsAdmin()) {
		a.forbidden(w, r, u, "PARTICIPANT_DENIED", i.Code, "Chỉ khai báo tỷ lệ chia thưởng khi hồ sơ còn nháp")
		return
	}
	action := r.FormValue("action")
	empNo := cleanCode(r.FormValue("employee_no"))
	if action == "remove" {
		if err := a.removeParticipant(i, empNo); err != nil {
			flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), err.Error())
			return
		}
		a.audit(r, u.EmployeeNo, "PARTICIPANT_REMOVE", i.Code, "200 OK", empNo)
		flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "Đã xóa đồng tác giả.")
		return
	}
	share, err := parsePercent(r.FormValue("share_percent"))
	if err != nil || share <= 0 {
		flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "Tỷ lệ chia thưởng phải lớn hơn 0 và không vượt 100.")
		return
	}
	roleNote := strings.TrimSpace(r.FormValue("role_note"))
	if roleNote == "" {
		roleNote = "Đồng tác giả"
	}
	if err := a.addParticipant(i, empNo, share, roleNote); err != nil {
		flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), err.Error())
		return
	}
	a.audit(r, u.EmployeeNo, "PARTICIPANT_ADD", i.Code, "200 OK", fmt.Sprintf("%s %.1f%%", empNo, share))
	flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "Đã cập nhật tỷ lệ chia thưởng.")
}

func (a *App) canContribute(u *User, i *Initiative, participants []Participant) bool {
	if u.IsAdmin() || i.SubmitterEmployeeNo == u.EmployeeNo {
		return true
	}
	if !u.CanViewWithParticipants(i, participants) {
		return false
	}
	for _, p := range participants {
		if p.EmployeeNo == u.EmployeeNo {
			return true
		}
	}
	return u.HasPerm(PermReviewROI) || u.HasPerm(PermApproveFinal)
}

func (a *App) attachmentPost(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	i, err := a.getInitiative(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	participants, _ := a.loadParticipants(id)
	if !u.CanViewWithParticipants(i, participants) {
		http.NotFound(w, r)
		return
	}
	if !a.canContribute(u, i, participants) {
		a.forbidden(w, r, u, "ATTACH_DENIED", i.Code, "Không được phép đính kèm hồ sơ này")
		return
	}
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "File quá lớn hoặc biểu mẫu không hợp lệ.")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "Vui lòng chọn file.")
		return
	}
	defer file.Close()
	docType := r.FormValue("document_type")
	if !a.validCode("md_document_type", docType) {
		docType = "OTHER"
	}
	original := safeFilename(hdr.Filename)
	ext := strings.ToLower(filepath.Ext(original))
	if !allowedUploadExt[ext] {
		flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "Định dạng file không được phép.")
		return
	}
	token, err := randomToken(16)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	dir := filepath.Join(a.UploadDir, i.Code)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	stored := token + ext
	path := filepath.Join(dir, stored)
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(out, h), io.LimitReader(file, (10<<20)+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(path)
		http.Error(w, "cannot save file", 500)
		return
	}
	if n > 10<<20 {
		os.Remove(path)
		flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "File vượt quá 10 MB.")
		return
	}
	if err := validateAttachmentFile(path, ext); err != nil {
		os.Remove(path)
		flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "Nội dung file không hợp lệ hoặc định dạng chưa được hỗ trợ.")
		return
	}
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	storageKey := filepath.ToSlash(filepath.Join(i.Code, stored))
	sha := hex.EncodeToString(h.Sum(nil))
	if _, err := a.DB.Exec(`
		INSERT INTO initiative_attachment(initiative_id,document_type_code,original_filename,storage_key,mime_type,size_bytes,sha256_hex,uploaded_by)
		VALUES(?,?,?,?,?,?,?,?)`, id, docType, original, storageKey, mimeType, n, sha, u.EmployeeNo); err != nil {
		os.Remove(path)
		http.Error(w, err.Error(), 500)
		return
	}
	a.audit(r, u.EmployeeNo, "ATTACHMENT_UPLOAD", i.Code, "200 OK",
		fmt.Sprintf("%s | %s | %s | %d bytes", original, docType, shortHash(sha), n))
	flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "Đã tải lên tài liệu, SHA-256 đã được lưu để đối chiếu.")
}

func (a *App) attachmentDownload(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var initiativeID int64
	var storage, original, mimeType, sha string
	err = a.DB.QueryRow(`
		SELECT initiative_id,storage_key,original_filename,COALESCE(mime_type,'application/octet-stream'),sha256_hex
		FROM initiative_attachment WHERE id=?`, id).Scan(&initiativeID, &storage, &original, &mimeType, &sha)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	i, err := a.getInitiative(initiativeID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	participants, _ := a.loadParticipants(initiativeID)
	if !u.CanViewWithParticipants(i, participants) {
		a.forbidden(w, r, u, "DOWNLOAD_DENIED", i.Code, "Ngoài phạm vi dữ liệu")
		return
	}
	absUpload, err := filepath.Abs(a.UploadDir)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	absPath, err := filepath.Abs(filepath.Join(a.UploadDir, filepath.FromSlash(storage)))
	if err != nil || !strings.HasPrefix(absPath, absUpload+string(os.PathSeparator)) {
		http.Error(w, "invalid storage key", 400)
		return
	}
	if _, err := os.Stat(absPath); err != nil {
		http.NotFound(w, r)
		return
	}
	a.audit(r, u.EmployeeNo, "ATTACHMENT_DOWNLOAD", i.Code, "200 OK", original+" | "+shortHash(sha))
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": original})
	if disposition == "" {
		disposition = "attachment"
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, absPath)
}
