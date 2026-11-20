package app

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (a *App) initiativeReviewPost(w http.ResponseWriter, r *http.Request, u *User) {
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
	perm := stagePerm(i.StageCode)
	if perm == "" {
		flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "Hồ sơ không ở bước có thể xử lý.")
		return
	}
	if !u.HasPerm(perm) {
		a.forbidden(w, r, u, "REVIEW_DENIED", i.Code, "Thiếu quyền "+perm)
		return
	}
	if !u.CanAct(i, perm) {
		a.forbidden(w, r, u, "REVIEW_SCOPE_DENIED", i.Code, "Ngoài phạm vi đơn vị hoặc hồ sơ đã đóng")
		return
	}
	if u.SelfReviewBlocked(i) {
		a.forbidden(w, r, u, "SELF_REVIEW_DENIED", i.Code, "Không được tự thẩm định/phê duyệt hồ sơ của mình")
		return
	}

	in := ReviewDecisionInput{
		Decision:             strings.TrimSpace(r.FormValue("decision")),
		Comment:              strings.TrimSpace(r.FormValue("comment")),
		GrossVND:             i.GrossValueVND,
		InvestVND:            i.InvestCostVND,
		Annex03:              i.Annex03,
		Basis:                i.RewardBasis,
		NonFinancialLevel:    i.NonFinancialLevel,
		ImplementationStatus: i.ImplementationStatusCode,
	}
	if len([]rune(in.Comment)) > 4000 {
		flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "Nội dung nhận xét quá dài (tối đa 4.000 ký tự).")
		return
	}

	if i.StageCode == StageReview {
		gross, okG, err := parseVND(r.FormValue("gross_value"))
		if err != nil {
			flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "Giá trị tiết kiệm không hợp lệ.")
			return
		}
		if okG {
			in.GrossVND = gross
		}
		invest, okI, err := parseVND(r.FormValue("invest_cost"))
		if err != nil {
			flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "Chi phí đầu tư không hợp lệ.")
			return
		}
		if okI {
			in.InvestVND = invest
		}
		in.Annex03 = r.FormValue("annex03") == "1" || r.FormValue("annex03") == "on"
		in.Basis = strings.TrimSpace(r.FormValue("reward_basis"))
		if in.Basis == "" {
			in.Basis = BasisFinancial
		}
		if in.Basis != BasisFinancial && in.Basis != BasisNonFinancial {
			flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "Cơ sở thưởng không hợp lệ.")
			return
		}
		if in.Basis == BasisNonFinancial {
			lvl, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("nonfinancial_level")))
			if lvl < 1 || lvl > 4 {
				flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "Chọn cấp độ thưởng phi tài chính Mục 5.2.")
				return
			}
			in.NonFinancialLevel = lvl
		} else {
			in.NonFinancialLevel = 0
		}
	}

	if i.StageCode == StageApprove && in.Decision == "APPROVE" {
		if raw := strings.TrimSpace(r.FormValue("override_reward")); raw != "" {
			v, _, err := parseVND(raw)
			if err != nil {
				flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "Số tiền phê duyệt không hợp lệ.")
				return
			}
			in.OverrideRewardVND = v
		}
	}

	if i.StageCode == StageImplement && in.Decision == "APPROVE" {
		impl := strings.TrimSpace(r.FormValue("implementation_status"))
		if impl != "IDEA" && impl != "PILOT" && impl != "IMPLEMENTED" {
			flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), "Chọn trạng thái triển khai thực tế.")
			return
		}
		in.ImplementationStatus = impl
		in.PaymentReference = strings.TrimSpace(r.FormValue("payment_ref"))
	}

	if err := a.applyStageReview(i, u.EmployeeNo, in); err != nil {
		flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), err.Error())
		return
	}
	detail := fmt.Sprintf("Bước %s · %s", i.StageName, in.Decision)
	if in.Decision == "APPROVE" && i.StageCode == StageReview {
		rw := ComputeReward(NetBenefit(in.GrossVND, in.InvestVND), in.Annex03, in.Basis, in.NonFinancialLevel)
		detail += fmt.Sprintf(" · Net %s · %s", formatMoney(NetBenefit(in.GrossVND, in.InvestVND)), rw.TierName)
	}
	if in.Comment != "" {
		detail += " · " + truncate(in.Comment, 240)
	}
	a.audit(r, u.EmployeeNo, "REVIEW_"+i.StageCode+"_"+in.Decision, i.Code, "200 OK", detail)

	msg := "Đã ghi nhận xử lý và chuyển bước."
	if in.Decision == "REJECT" {
		msg = "Đã từ chối hồ sơ; hồ sơ được lưu vết, không bị xóa."
	}
	if in.Decision == "NEED_INFO" {
		msg = "Đã ghi nhận yêu cầu bổ sung; hồ sơ giữ nguyên bước."
	}
	flashRedirect(w, r, fmt.Sprintf("/initiatives/%d", id), msg)
}

func truncate(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}
