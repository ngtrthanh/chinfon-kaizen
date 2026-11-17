package app

import (
	"fmt"
	"math"
	"strings"
)

const (
	TrackFast48       = "FAST48"
	TrackStandard     = "STANDARD"
	TrackConfidential = "CONFIDENTIAL"

	StageDraft     = "DRAFT"
	StageScreen    = "SCREEN"
	StageReview    = "REVIEW"
	StageApprove   = "APPROVE"
	StageImplement = "IMPLEMENT"
	StageApplied   = "APPLIED"

	StatusOpen     = "OPEN"
	StatusRejected = "REJECTED"
	StatusClosed   = "CLOSED"

	BasisFinancial    = "FINANCIAL"
	BasisNonFinancial = "NONFINANCIAL"

	PermCreate       = "idea:create"
	PermScreen       = "idea:screen:dept"
	PermReviewROI    = "idea:review:roi"
	PermApproveFinal = "idea:approve:final"
	PermConfidential = "track:confidential:read"
	PermPayroll      = "hr:payroll:export"
	PermRBACAdmin    = "rbac:admin"

	RoleCEOAudit   = "CEO_AUDIT"
	RoleCouncil    = "COUNCIL"
	RoleDirector   = "DIRECTOR"
	RoleManager    = "MANAGER"
	RoleSupervisor = "SUPERVISOR"
	RoleHRD        = "HRD"
	RoleAdmin      = "ADMIN"
	RoleWorker     = "WORKER"

	MaxRewardVND  = int64(100_000_000)
	SeedRewardVND = int64(300_000)
)

type RewardResult struct {
	TierCode  string
	TierName  string
	BaseVND   int64
	AmountVND int64
	Note      string
}

// ComputeFinancialReward is the authoritative implementation of PN-B0000-02
// Mục 5.1 (7 tiers). All money is integer VND; there is no floating storage.
func ComputeFinancialReward(netBenefitVND int64, annex03 bool) RewardResult {
	var base int64
	var code, name, note string

	switch {
	case netBenefitVND <= 0:
		base = SeedRewardVND
		code, name = "T1_IDEA", "Mức 1 — Thưởng khuyến khích ý tưởng"
		note = "Tặng đứt khi ý tưởng được duyệt thử nghiệm."
	case netBenefitVND < 20_000_000:
		base = 1_000_000
		code, name = "T2", "Mức 2 — Khen ngợi Cấp 1"
		note = "Làm lợi ròng < 20 triệu VNĐ/năm."
	case netBenefitVND < 50_000_000:
		base = 2_500_000
		code, name = "T3", "Mức 3 — Khen ngợi Cấp 2"
		note = "Làm lợi ròng 20 đến < 50 triệu VNĐ/năm."
	case netBenefitVND < 150_000_000:
		base = 5_000_000
		code, name = "T4", "Mức 4 — Ghi công Nhỏ Cấp 1"
		note = "Làm lợi ròng 50 đến < 150 triệu VNĐ/năm."
	case netBenefitVND < 300_000_000:
		base = 10_000_000
		code, name = "T5", "Mức 5 — Ghi công Nhỏ Cấp 2"
		note = "Làm lợi ròng 150 đến < 300 triệu VNĐ/năm."
	case netBenefitVND < 500_000_000:
		base = 20_000_000
		code, name = "T6", "Mức 6 — Ghi công Lớn"
		note = "Làm lợi ròng 300 đến < 500 triệu VNĐ/năm."
	default:
		base = int64(math.Min(float64(MaxRewardVND), math.Round(float64(netBenefitVND)*0.06)))
		code, name = "T7", "Mức 7 — Giải thưởng Đột phá"
		note = "6% lợi ích ròng năm đầu, tối đa 100 triệu VNĐ."
	}

	result := RewardResult{TierCode: code, TierName: name, BaseVND: base, AmountVND: base, Note: note}
	if annex03 {
		result.AmountVND = int64(math.Min(float64(MaxRewardVND), math.Round(float64(base)*1.2)))
		result.TierName += " • Hệ số 1.2x Phụ lục 03"
		result.Note += " Đã nhân hệ số 1.2x theo Phụ lục 03."
	}
	return result
}

func NonFinancialReward(level int) RewardResult {
	tiers := []RewardResult{
		{},
		{TierCode: "NF1", TierName: "Mục 5.2 — Mức 1 (cục bộ)", BaseVND: 1_000_000, AmountVND: 1_000_000,
			Note: "Rò rỉ dầu/khí, kẹt con lăn, ảnh hưởng cục bộ."},
		{TierCode: "NF2", TierName: "Mục 5.2 — Mức 2 (dây chuyền)", BaseVND: 3_000_000, AmountVND: 3_000_000,
			Note: "Ngăn dừng lò/nghiền < 24h, vượt tiến độ đại tu."},
		{TierCode: "NF3", TierName: "Mục 5.2 — Mức 3 (nghiêm trọng)", BaseVND: 8_000_000, AmountVND: 8_000_000,
			Note: "Ngăn cháy nổ trạm điện/kho than, gian lận vật tư."},
		{TierCode: "NF4", TierName: "Mục 5.2 — Mức 4 (đặc biệt)", BaseVND: 25_000_000, AmountVND: 25_000_000,
			Note: "Phá đường dây tiêu cực lớn, cứu hộ; TGĐ quyết định."},
	}
	if level < 1 || level >= len(tiers) {
		return tiers[0]
	}
	return tiers[level]
}

func ComputeReward(netBenefitVND int64, annex03 bool, basis string, nonFinancialLevel int) RewardResult {
	if basis == BasisNonFinancial {
		return NonFinancialReward(nonFinancialLevel)
	}
	return ComputeFinancialReward(netBenefitVND, annex03)
}

func NetBenefit(gross, invest int64) int64 {
	if gross < 0 {
		gross = 0
	}
	if invest < 0 {
		invest = 0
	}
	if gross <= invest {
		return 0
	}
	return gross - invest
}

func formatMoney(v int64) string {
	s := fmt.Sprintf("%d", v)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	if neg {
		s = "-" + s
	}
	return s + " VNĐ"
}

func formatShortMoney(v int64) string {
	abs := v
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs >= 1_000_000_000:
		return fmt.Sprintf("%.2f Tỷ", float64(v)/1_000_000_000)
	case abs >= 1_000_000:
		return fmt.Sprintf("%.1f Tr", float64(v)/1_000_000)
	case abs >= 1_000:
		return fmt.Sprintf("%.0f K", float64(v)/1_000)
	default:
		return fmt.Sprintf("%d", v)
	}
}

func stagePerm(stage string) string {
	switch stage {
	case StageScreen:
		return PermScreen
	case StageReview:
		return PermReviewROI
	case StageApprove:
		return PermApproveFinal
	case StageImplement:
		return PermPayroll
	default:
		return ""
	}
}

func stageActionLabel(stage string) string {
	switch stage {
	case StageScreen:
		return "Xác nhận hiện trường"
	case StageReview:
		return "Thẩm định & chuyển Hội đồng/TGĐ"
	case StageApprove:
		return "Phê duyệt quyết định thưởng"
	case StageImplement:
		return "Xác nhận triển khai & chi trả"
	default:
		return "Xử lý"
	}
}

func StageName(code string) string {
	switch code {
	case StageDraft:
		return "Nháp"
	case StageScreen:
		return "Xác nhận hiện trường"
	case StageReview:
		return "Thẩm định & nghiệm thu"
	case StageApprove:
		return "Phê duyệt quyết định thưởng"
	case StageImplement:
		return "Triển khai & xác nhận kết quả"
	case StageApplied:
		return "Đã chi trả & vinh danh"
	default:
		return code
	}
}

func TrackName(code string) string {
	switch code {
	case TrackFast48:
		return "Luồng 1 — Thưởng nóng 48h"
	case TrackStandard:
		return "Luồng 2 — Tiêu chuẩn"
	case TrackConfidential:
		return "Luồng 3 — Bảo mật tố giác"
	default:
		return code
	}
}
