package app

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func csvSafe(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

func (a *App) rewardsExport(w http.ResponseWriter, r *http.Request, u *User) {
	if !u.HasPerm(PermPayroll) && !u.IsAdmin() {
		a.forbidden(w, r, u, "EXPORT_DENIED", "/rewards/export.csv", "Thiếu quyền hr:payroll:export")
		return
	}
	rows, totals, err := a.rewardLedger(u)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	filename := fmt.Sprintf("Chinfon_Kaizen_Rewards_%s.csv", time.Now().Format("20060102"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

	cw := csv.NewWriter(w)
	cw.UseCRLF = true
	_ = cw.Write([]string{
		"Mã hồ sơ", "Mã nhân viên", "Họ tên", "Đơn vị", "Tỷ lệ chia thưởng (%)",
		"Số tiền (VND)", "Mức thưởng", "Trạng thái chi trả", "Ngày chi trả",
	})
	for _, row := range rows {
		status := "Chờ chi trả"
		paid := ""
		if row.PaidAt.Valid {
			status = "Đã chi trả"
			paid = formatDate(row.PaidAt.String)
		}
		_ = cw.Write([]string{
			csvSafe(row.Code),
			csvSafe(row.EmployeeNo),
			csvSafe(row.EmployeeName),
			csvSafe(row.DepartmentName),
			fmt.Sprintf("%.1f", row.SharePercent),
			fmt.Sprintf("%d", row.AmountVND),
			csvSafe(row.TierName),
			status,
			paid,
		})
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		a.Logger.Printf("csv export: %v", err)
	}
	a.audit(r, u.EmployeeNo, "PAYROLL_EXPORT", filename, "200 OK",
		fmt.Sprintf("approved=%d paid=%d pending=%d rows=%d", totals.ApprovedVND, totals.PaidVND, totals.PendingVND, len(rows)))
}
