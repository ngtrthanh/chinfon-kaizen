package app

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"chinfon-kaizen/internal/security"
)

func (a *App) requireAdmin(w http.ResponseWriter, r *http.Request, u *User, target string) bool {
	if u.IsAdmin() {
		return true
	}
	a.forbidden(w, r, u, "ADMIN_DENIED", target, "Thiếu quyền rbac:admin")
	return false
}

func adminRedirect(w http.ResponseWriter, r *http.Request, tab, msg, errMsg string) {
	v := url.Values{}
	v.Set("tab", tab)
	if msg != "" {
		v.Set("msg", msg)
	}
	if errMsg != "" {
		v.Set("err", errMsg)
	}
	http.Redirect(w, r, "/admin?"+v.Encode(), http.StatusSeeOther)
}

func (a *App) adminPage(w http.ResponseWriter, r *http.Request, u *User) {
	if !a.requireAdmin(w, r, u, "/admin") {
		return
	}
	tab := strings.TrimSpace(r.URL.Query().Get("tab"))
	if !validAdminTab(tab) {
		tab = "master"
	}
	d, err := a.adminPageData(u, r, tab)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	applyFlash(&d, r)
	a.render(w, "admin", d)
}

func validAdminTab(tab string) bool {
	switch tab {
	case "master", "users", "rbac", "audit", "staff":
		return true
	}
	return false
}

func (a *App) adminPageData(u *User, r *http.Request, tab string) (PageData, error) {
	depts, err := a.masterDepartments()
	if err != nil {
		return PageData{}, err
	}
	emps, err := a.masterEmployees()
	if err != nil {
		return PageData{}, err
	}
	users, err := a.masterUsers()
	if err != nil {
		return PageData{}, err
	}
	roles, _ := a.roles()
	perms, _ := a.permissions()
	matrix, _ := a.permissionMatrix()
	scopes, _ := a.scopes()
	audits, _ := a.auditList(r.URL.Query().Get("q"), 250)

	d := a.basePage(u, "Cài đặt hệ thống & RBAC", "cem-auth · master data · nhật ký bất biến", "admin")
	d.Tab = tab
	d.DepartmentMaster = depts
	d.Departments, _ = a.options("md_department")
	d.Employees = emps
	d.Users = users
	d.Roles = roles
	d.Permissions = perms
	d.Matrix = matrix
	d.Scopes = scopes
	d.Audit = audits
	d.Filters = map[string]string{"q": r.URL.Query().Get("q")}

	if tab == "staff" {
		staff, err := a.staffRows()
		if err != nil {
			return PageData{}, err
		}
		d.Staff = staff
		d.StaffCount = len(staff)
		for _, s := range staff {
			if s.HasAccount {
				d.StaffAccountCount++
			}
			if s.MustChange {
				d.StaffMustChangeCount++
			}
			if s.HasAccount && s.Active && contains(s.Roles, RoleCouncil) {
				d.HasCouncil = true
			}
		}
	}
	return d, nil
}

func (a *App) adminDepartmentPost(w http.ResponseWriter, r *http.Request, u *User) {
	if !a.requireAdmin(w, r, u, "/admin/departments") {
		return
	}
	code := cleanCode(r.FormValue("code"))
	name := strings.TrimSpace(r.FormValue("name"))
	kind := cleanCode(r.FormValue("kind"))
	sortOrder, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("sort_order")))
	if sortOrder <= 0 {
		sortOrder = 100
	}
	switch kind {
	case "PLANT", "SUPPORT", "HRD", "BOD", "BRANCH":
	default:
		kind = "PLANT"
	}
	if code == "" || name == "" {
		adminRedirect(w, r, "master", "", "Mã và tên đơn vị là bắt buộc.")
		return
	}
	_, err := a.DB.Exec(`INSERT INTO md_department(code,name,kind,sort_order,active,is_demo) VALUES(?,?,?,?,1,0)
		ON CONFLICT(code) DO UPDATE SET name=excluded.name,kind=excluded.kind,sort_order=excluded.sort_order,active=1,is_demo=0`,
		code, name, kind, sortOrder)
	if err != nil {
		adminRedirect(w, r, "master", "", "Không lưu được đơn vị: "+err.Error())
		return
	}
	a.audit(r, u.EmployeeNo, "MASTER_DEPARTMENT_UPSERT", code, "200 OK", name+" · "+kind)
	adminRedirect(w, r, "master", "Đã lưu đơn vị "+code, "")
}

func (a *App) adminDepartmentToggle(w http.ResponseWriter, r *http.Request, u *User) {
	if !a.requireAdmin(w, r, u, "/admin/departments/toggle") {
		return
	}
	code := cleanCode(r.PathValue("code"))
	if code == "" {
		adminRedirect(w, r, "master", "", "Mã đơn vị không hợp lệ.")
		return
	}
	res, err := a.DB.Exec(`UPDATE md_department SET active = CASE active WHEN 1 THEN 0 ELSE 1 END WHERE code=?`, code)
	if err != nil {
		adminRedirect(w, r, "master", "", err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		adminRedirect(w, r, "master", "", "Không tìm thấy đơn vị.")
		return
	}
	a.audit(r, u.EmployeeNo, "MASTER_DEPARTMENT_TOGGLE", code, "200 OK", "Đổi trạng thái hoạt động")
	adminRedirect(w, r, "master", "Đã đổi trạng thái đơn vị "+code, "")
}

func (a *App) adminEmployeePost(w http.ResponseWriter, r *http.Request, u *User) {
	if !a.requireAdmin(w, r, u, "/admin/employees") {
		return
	}
	no := cleanCode(r.FormValue("employee_no"))
	name := strings.TrimSpace(r.FormValue("full_name"))
	dept := cleanCode(r.FormValue("department_code"))
	position := strings.TrimSpace(r.FormValue("position_title"))
	group := strings.TrimSpace(r.FormValue("work_group"))
	if no == "" || name == "" || !a.validCode("md_department", dept) {
		adminRedirect(w, r, "master", "", "Mã nhân viên, họ tên và đơn vị hợp lệ là bắt buộc.")
		return
	}
	_, err := a.DB.Exec(`INSERT INTO md_employee(employee_no,full_name,department_code,position_title,work_group,active,is_demo) VALUES(?,?,?,?,?,1,0)
		ON CONFLICT(employee_no) DO UPDATE SET full_name=excluded.full_name,department_code=excluded.department_code,
			position_title=excluded.position_title,work_group=excluded.work_group,active=1,is_demo=0`, no, name, dept, position, group)
	if err != nil {
		adminRedirect(w, r, "master", "", "Không lưu được nhân viên: "+err.Error())
		return
	}
	a.audit(r, u.EmployeeNo, "MASTER_EMPLOYEE_UPSERT", no, "200 OK", name+" · "+dept)
	adminRedirect(w, r, "master", "Đã lưu nhân viên "+no, "")
}

func (a *App) adminEmployeeToggle(w http.ResponseWriter, r *http.Request, u *User) {
	if !a.requireAdmin(w, r, u, "/admin/employees/toggle") {
		return
	}
	no := cleanCode(r.PathValue("no"))
	if no == u.EmployeeNo {
		adminRedirect(w, r, "master", "", "Không thể ngừng hoạt động nhân viên đang đăng nhập.")
		return
	}
	res, err := a.DB.Exec(`UPDATE md_employee SET active = CASE active WHEN 1 THEN 0 ELSE 1 END WHERE employee_no=?`, no)
	if err != nil {
		adminRedirect(w, r, "master", "", err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		adminRedirect(w, r, "master", "", "Không tìm thấy nhân viên.")
		return
	}
	a.audit(r, u.EmployeeNo, "MASTER_EMPLOYEE_TOGGLE", no, "200 OK", "Đổi trạng thái hoạt động")
	adminRedirect(w, r, "master", "Đã đổi trạng thái nhân viên "+no, "")
}

func (a *App) validScope(scope string) bool {
	return scope == "ALL" || a.validCode("md_department", scope)
}

func (a *App) adminUserCreate(w http.ResponseWriter, r *http.Request, u *User) {
	if !a.requireAdmin(w, r, u, "/admin/users") {
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	employeeNo := cleanCode(r.FormValue("employee_no"))
	password := r.FormValue("password")
	scope := strings.TrimSpace(r.FormValue("scope"))
	roles := r.Form["roles"]
	if username == "" || employeeNo == "" || password == "" {
		adminRedirect(w, r, "users", "", "Tên đăng nhập, nhân viên và mật khẩu là bắt buộc.")
		return
	}
	if len(username) < 3 || len(username) > 40 || strings.ContainsAny(username, " \t\r\n") {
		adminRedirect(w, r, "users", "", "Tên đăng nhập 3–40 ký tự, không khoảng trắng.")
		return
	}
	if scope == "" || !a.validScope(scope) {
		adminRedirect(w, r, "users", "", "Phạm vi dữ liệu không hợp lệ.")
		return
	}
	if len(roles) == 0 {
		adminRedirect(w, r, "users", "", "Phải gán ít nhất một vai trò.")
		return
	}
	for _, role := range roles {
		if !a.validCode("rbac_role", role) {
			adminRedirect(w, r, "users", "", "Vai trò không hợp lệ: "+role)
			return
		}
	}
	var employeeCount int
	if err := a.DB.QueryRow(`SELECT COUNT(*) FROM md_employee WHERE employee_no=? AND active=1`, employeeNo).Scan(&employeeCount); err != nil || employeeCount == 0 {
		adminRedirect(w, r, "users", "", "Nhân viên không tồn tại hoặc đã ngừng hoạt động.")
		return
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		adminRedirect(w, r, "users", "", err.Error())
		return
	}
	id, err := a.createUser(username, hash, employeeNo, scope, roles)
	if err != nil {
		adminRedirect(w, r, "users", "", "Không tạo được tài khoản: "+err.Error())
		return
	}
	a.audit(r, u.EmployeeNo, "USER_CREATE", username, "201 CREATED",
		fmt.Sprintf("id=%d · %s · roles=%s · scope=%s", id, employeeNo, strings.Join(roles, ","), scope))
	adminRedirect(w, r, "users", "Đã tạo tài khoản "+username, "")
}

func (a *App) adminUserUpdate(w http.ResponseWriter, r *http.Request, u *User) {
	if !a.requireAdmin(w, r, u, "/admin/users/update") {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		adminRedirect(w, r, "users", "", "Tài khoản không hợp lệ.")
		return
	}
	var employeeNo string
	if err := a.DB.QueryRow(`SELECT employee_no FROM app_user WHERE id=?`, id).Scan(&employeeNo); err != nil {
		adminRedirect(w, r, "users", "", "Không tìm thấy tài khoản.")
		return
	}
	scope := strings.TrimSpace(r.FormValue("scope"))
	if scope == "" || !a.validScope(scope) {
		adminRedirect(w, r, "users", "", "Phạm vi dữ liệu không hợp lệ.")
		return
	}
	roles := r.Form["roles"]
	if len(roles) == 0 {
		adminRedirect(w, r, "users", "", "Phải giữ ít nhất một vai trò.")
		return
	}
	for _, role := range roles {
		if !a.validCode("rbac_role", role) {
			adminRedirect(w, r, "users", "", "Vai trò không hợp lệ: "+role)
			return
		}
	}
	active := r.FormValue("active") == "1"
	if id == u.ID && !active {
		adminRedirect(w, r, "users", "", "Không thể tự vô hiệu hóa tài khoản đang đăng nhập.")
		return
	}
	if !active || !contains(roles, RoleAdmin) {
		var currentlyAdmin int
		_ = a.DB.QueryRow(`SELECT COUNT(*) FROM app_user_role WHERE user_id=? AND role_code='ADMIN'`, id).Scan(&currentlyAdmin)
		if currentlyAdmin > 0 {
			admins, _ := a.countAdmins()
			if admins <= 1 {
				adminRedirect(w, r, "users", "", "Không thể thu hồi quyền ADMIN cuối cùng.")
				return
			}
		}
	}
	if err := a.updateUser(id, scope, roles, active); err != nil {
		adminRedirect(w, r, "users", "", "Không cập nhật được tài khoản: "+err.Error())
		return
	}
	a.audit(r, u.EmployeeNo, "USER_UPDATE", employeeNo, "200 OK",
		fmt.Sprintf("id=%d · roles=%s · scope=%s · active=%v", id, strings.Join(roles, ","), scope, active))
	adminRedirect(w, r, "users", "Đã cập nhật tài khoản "+employeeNo, "")
}

func (a *App) adminUserPassword(w http.ResponseWriter, r *http.Request, u *User) {
	if !a.requireAdmin(w, r, u, "/admin/users/password") {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		adminRedirect(w, r, "users", "", "Tài khoản không hợp lệ.")
		return
	}
	password := r.FormValue("new_password")
	var employeeNo string
	if err := a.DB.QueryRow(`SELECT employee_no FROM app_user WHERE id=?`, id).Scan(&employeeNo); err != nil {
		adminRedirect(w, r, "users", "", "Không tìm thấy tài khoản.")
		return
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		adminRedirect(w, r, "users", "", err.Error())
		return
	}
	csrf, err := randomToken(24)
	if err != nil {
		adminRedirect(w, r, "users", "", err.Error())
		return
	}
	if _, err := a.DB.Exec(`UPDATE app_user SET password_hash=?, must_change_password=1, csrf_token=? WHERE id=?`, hash, csrf, id); err != nil {
		adminRedirect(w, r, "users", "", "Không đặt lại được mật khẩu: "+err.Error())
		return
	}
	_, _ = a.DB.Exec(`DELETE FROM app_session WHERE user_id=?`, id)
	a.audit(r, u.EmployeeNo, "USER_PASSWORD_RESET", employeeNo, "200 OK", "ADMIN đặt lại mật khẩu và thu hồi phiên")
	adminRedirect(w, r, "users", "Đã đặt lại mật khẩu cho "+employeeNo, "")
}

func (a *App) adminRBACToggle(w http.ResponseWriter, r *http.Request, u *User) {
	if !a.requireAdmin(w, r, u, "/admin/rbac") {
		return
	}
	role := cleanCode(r.FormValue("role"))
	perm := strings.TrimSpace(r.FormValue("permission"))
	allow := r.FormValue("allow") == "1"
	if !a.validCode("rbac_role", role) || !a.validCode("rbac_permission", perm) {
		adminRedirect(w, r, "rbac", "", "Vai trò hoặc quyền không hợp lệ.")
		return
	}
	if role == RoleAdmin && perm == PermRBACAdmin && !allow {
		adminRedirect(w, r, "rbac", "", "Không thể thu hồi quyền quản trị cuối cùng của ADMIN.")
		return
	}
	if err := a.setRolePermission(role, perm, allow); err != nil {
		adminRedirect(w, r, "rbac", "", err.Error())
		return
	}
	state := "GRANT"
	if !allow {
		state = "REVOKE"
	}
	a.audit(r, u.EmployeeNo, "RBAC_"+state, role+" / "+perm, "200 OK", "Cập nhật ma trận quyền")
	adminRedirect(w, r, "rbac", "Đã cập nhật quyền "+perm+" cho "+role, "")
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
