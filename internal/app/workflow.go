package app

import (
	"database/sql"
	"fmt"
	"strings"
)

type ReviewDecisionInput struct {
	Decision             string
	Comment              string
	GrossVND             int64
	InvestVND            int64
	Annex03              bool
	Basis                string
	NonFinancialLevel    int
	OverrideRewardVND    int64
	ImplementationStatus string
	PaymentReference string // External approved payroll/payment evidence; not proof the bank settled.
}

func nextStage(stage string) string {
	switch stage {
	case StageScreen:
		return StageReview
	case StageReview:
		return StageApprove
	case StageApprove:
		return StageImplement
	case StageImplement:
		return StageApplied
	default:
		return ""
	}
}

// applyStageReview is the transactional heart of PN-B0000-02: it records the
// review, updates valuation, guards the current stage (optimistic lock), appends
// immutable history and moves the record exactly once.
func (a *App) applyStageReview(i *Initiative, actor string, in ReviewDecisionInput) error {
	if in.Decision != "APPROVE" && in.Decision != "REJECT" && in.Decision != "NEED_INFO" {
		return fmt.Errorf("quyết định không hợp lệ")
	}
	to := nextStage(i.StageCode)
	if to == "" {
		return fmt.Errorf("hồ sơ không ở bước có thể xử lý")
	}

	// A workflow approval is not itself a disbursement. HR must record
	// a traceable external payment document before stage APPLIED is entered.
	if i.StageCode == StageImplement && in.Decision == "APPROVE" {
		ref := strings.TrimSpace(in.PaymentReference)
		if len(ref) < 6 || len(ref) > 120 || strings.ContainsAny(ref, "\r\n\t") {
			return fmt.Errorf("bắt buộc nhập mã chứng từ chi trả (6–120 ký tự) đã được HR đối chiếu")
		}
		in.PaymentReference = ref
	}

	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := nowUTC()
	rewardTier := sql.NullString{}
	rewardAmount := int64(0)
	var assessedGross, assessedInvest, assessedNet sql.NullInt64
	implStatus := i.ImplementationStatusCode
	if i.StageCode == StageReview || i.StageCode == StageApprove || i.StageCode == StageImplement {
		rewardTier = sql.NullString{String: i.RewardTierCode, Valid: i.RewardTierCode != ""}
		rewardAmount = i.RewardAmountVND
		assessedGross = sql.NullInt64{Int64: i.GrossValueVND, Valid: true}
		assessedInvest = sql.NullInt64{Int64: i.InvestCostVND, Valid: true}
		assessedNet = sql.NullInt64{Int64: i.NetBenefitVND, Valid: true}
	}

	if i.StageCode == StageReview {
		if in.Basis != BasisFinancial && in.Basis != BasisNonFinancial {
			in.Basis = BasisFinancial
		}
		net := NetBenefit(in.GrossVND, in.InvestVND)
		reward := ComputeReward(net, in.Annex03, in.Basis, in.NonFinancialLevel)
		annex := 0
		if in.Annex03 {
			annex = 1
		}
		if _, err := tx.Exec(`
			UPDATE initiative SET gross_value_vnd=?,invest_cost_vnd=?,net_benefit_vnd=?,annex03=?,
				reward_basis=?,nonfinancial_level=?,reward_tier_code=?,base_reward_vnd=?,reward_amount_vnd=?,updated_at=?
			WHERE id=? AND stage_code=? AND record_status='OPEN'`,
			in.GrossVND, in.InvestVND, net, annex, in.Basis, in.NonFinancialLevel,
			reward.TierCode, reward.BaseVND, reward.AmountVND, now, i.ID, i.StageCode); err != nil {
			return err
		}
		rewardTier = sql.NullString{String: reward.TierCode, Valid: true}
		rewardAmount = reward.AmountVND
		assessedGross = sql.NullInt64{Int64: in.GrossVND, Valid: true}
		assessedInvest = sql.NullInt64{Int64: in.InvestVND, Valid: true}
		assessedNet = sql.NullInt64{Int64: net, Valid: true}
	}

	if i.StageCode == StageApprove && in.Decision == "APPROVE" && in.OverrideRewardVND > 0 {
		if in.OverrideRewardVND > MaxRewardVND {
			return fmt.Errorf("số tiền vượt trần 100.000.000 VNĐ")
		}
		if _, err := tx.Exec(`UPDATE initiative SET reward_amount_vnd=?,updated_at=? WHERE id=? AND stage_code=? AND record_status='OPEN'`,
			in.OverrideRewardVND, now, i.ID, i.StageCode); err != nil {
			return err
		}
		rewardAmount = in.OverrideRewardVND
	}

	if i.StageCode == StageImplement && in.Decision == "APPROVE" {
		if in.ImplementationStatus != "IDEA" && in.ImplementationStatus != "PILOT" && in.ImplementationStatus != "IMPLEMENTED" {
			return fmt.Errorf("trạng thái triển khai không hợp lệ")
		}
		implStatus = in.ImplementationStatus
	}

	if _, err := tx.Exec(`
		INSERT INTO initiative_review(initiative_id,stage_code,reviewer_employee_no,decision,comment,
			assessed_gross_vnd,assessed_invest_vnd,assessed_net_vnd,reward_tier_code,reward_amount_vnd,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		i.ID, i.StageCode, actor, in.Decision, in.Comment,
		assessedGross, assessedInvest, assessedNet, rewardTier, rewardAmount, now); err != nil {
		return err
	}

	switch in.Decision {
	case "APPROVE":
		action := "APPROVED"
		switch i.StageCode {
		case StageScreen:
			action = "SCREENED"
		case StageReview:
			action = "REVIEWED"
		case StageApprove:
			action = "FINAL_APPROVED"
		case StageImplement:
			action = "PAID"
		}
		var res sql.Result
		if i.StageCode == StageImplement {
			res, err = tx.Exec(`UPDATE initiative SET stage_code=?,implementation_status_code=?,paid_at=?,paid_by=?,payment_reference=?,updated_at=?
				WHERE id=? AND stage_code=? AND record_status='OPEN'`,
				to, implStatus, now, actor, in.PaymentReference, now, i.ID, i.StageCode)
		} else {
			res, err = tx.Exec(`UPDATE initiative SET stage_code=?,updated_at=? WHERE id=? AND stage_code=? AND record_status='OPEN'`,
				to, now, i.ID, i.StageCode)
		}
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			return fmt.Errorf("trạng thái hồ sơ đã thay đổi, vui lòng tải lại")
		}
		if _, err := tx.Exec(`INSERT INTO initiative_history(initiative_id,from_stage_code,to_stage_code,action_code,changed_by,note,created_at)
			VALUES(?,?,?,?,?,?,?)`, i.ID, i.StageCode, to, action, actor, func() string {
				if i.StageCode == StageImplement { return strings.TrimSpace(in.Comment) + " [payment_ref=" + in.PaymentReference + "]" }
				return in.Comment
			}(), now); err != nil {
			return err
		}
	case "REJECT":
		if strings.TrimSpace(in.Comment) == "" {
			return fmt.Errorf("phải ghi rõ lý do từ chối")
		}
		res, err := tx.Exec(`UPDATE initiative SET record_status='REJECTED',reject_reason=?,updated_at=?
			WHERE id=? AND stage_code=? AND record_status='OPEN'`, in.Comment, now, i.ID, i.StageCode)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			return fmt.Errorf("trạng thái hồ sơ đã thay đổi, vui lòng tải lại")
		}
		if _, err := tx.Exec(`INSERT INTO initiative_history(initiative_id,from_stage_code,to_stage_code,action_code,changed_by,note,created_at)
			VALUES(?,?,?,'REJECTED',?,?,?)`, i.ID, i.StageCode, i.StageCode, actor, in.Comment, now); err != nil {
			return err
		}
	case "NEED_INFO":
		if strings.TrimSpace(in.Comment) == "" {
			return fmt.Errorf("phải ghi rõ nội dung cần bổ sung")
		}
		if _, err := tx.Exec(`INSERT INTO initiative_history(initiative_id,from_stage_code,to_stage_code,action_code,changed_by,note,created_at)
			VALUES(?,?,?,'NEED_INFO',?,?,?)`, i.ID, i.StageCode, i.StageCode, actor, in.Comment, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}
