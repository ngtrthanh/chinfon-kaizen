package app

import (
	"database/sql"
	"fmt"
	"time"
)

type demoInitiative struct {
	Track, Title, Dept, Submitter, Equipment string
	ProblemSummary, ProblemDetail            string
	SolutionSummary, SolutionDetail          string
	BenefitSummary                           string
	Stage, ImplStatus                        string
	Gross, Invest                            int64
	Annex03                                  bool
	Split                                    string
	Participants                             map[string]float64
	DaysAgo                                  int
}

// seedDemoData creates illustrative records only when the initiative table is
// empty. It never creates user accounts or default passwords.
func seedDemoData(db *sql.DB) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM initiative`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	var empCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM md_employee`).Scan(&empCount); err != nil || empCount == 0 {
		return fmt.Errorf("master employees missing")
	}

	year := time.Now().Year()
	demos := []demoInitiative{
		{
			Track: TrackFast48, Stage: StageApprove, ImplStatus: "PILOT",
			Title: "Tự chế tạo dàn phun khí nén giải nhiệt điểm đỏ vỏ lò 1K1-KILN01",
			Dept:  "KILN", Submitter: "CF-0412", Equipment: "1K1-KILN01",
			ProblemSummary:  "Điểm đỏ 415°C trên vỏ lò quay số 1 có nguy cơ dừng lò đột xuất.",
			ProblemDetail:   "Camera nhiệt ghi nhận điểm nóng cục bộ 415°C tại vùng tiếp giáp vỏ lò, vượt ngưỡng an toàn. Phương án phun nước cưỡng bức gây sốc nhiệt và làm giảm tuổi thọ gạch chịu lửa.",
			SolutionSummary: "Dàn vòi phun khí nén may đo kết hợp camera nhiệt giám sát.",
			SolutionDetail:  "Tổ Cơ khí tự thiết kế cơ cấu giá đỡ và dàn vòi phun khí nén, bố trí theo bản đồ nhiệt; tự lập trình chu kỳ phun theo tín hiệu camera nhiệt, không mua thiết bị thay thế 1-1.",
			BenefitSummary:  "Giảm 48 giờ dừng lò/năm và kéo dài tuổi thọ vỏ lò; làm lợi ròng 880 triệu VNĐ/năm.",
			Gross:           920_000_000, Invest: 40_000_000, Annex03: true, DaysAgo: 2,
			Split:        "60% Tác giả chính / 40% Đồng tác giả",
			Participants: map[string]float64{"CF-0118": 40},
		},
		{
			Track: TrackConfidential, Stage: StageReview, ImplStatus: "IDEA",
			Title: "[Bảo mật] Phát hiện gian lận xả nước dằn xe bồn tại Trạm cân số 2",
			Dept:  "LOGISTICS", Submitter: "CF-0911", Equipment: "WB-SCALE-02",
			ProblemSummary:  "Nguy cơ thất thoát xi măng rời qua gian lận khối lượng tại trạm cân.",
			ProblemDetail:   "Bằng chứng video và log trạm cân cho thấy 03 xe bồn xả nước dằn trước khi cân, chênh lệch khoảng 1,2 tấn/chuyến. Nội dung được chuyển thẳng Tổng Giám đốc và Kiểm toán Nội bộ.",
			SolutionSummary: "Khóa quy trình cân, đối chiếu tự động log cân – camera – hóa đơn vận tải.",
			SolutionDetail:  "Đề xuất hiệu chỉnh quy trình kiểm soát tại trạm cân số 2, bổ sung đối chiếu tự động và chế tài với nhà vận tải vi phạm.",
			BenefitSummary:  "Ngăn thất thoát ước tính 1,565 tỷ VNĐ/năm.",
			Gross:           1_650_000_000, Invest: 85_000_000, Annex03: true, DaysAgo: 1,
			Split: "100% Người cung cấp bằng chứng",
		},
		{
			Track: TrackStandard, Stage: StageReview, ImplStatus: "PILOT",
			Title: "Tối ưu phối liệu tăng tỷ lệ xỉ hạt lò cao & tro bay tại Hiệp Phước",
			Dept:  "GRIND", Submitter: "CF-0519", Equipment: "HP-MILL-01",
			ProblemSummary:  "Tỷ lệ Clinker trong PCB40 còn cao, chi phí nguyên liệu đầu vào lớn.",
			ProblemDetail:   "Độ mịn Blaine và phụ gia trợ nghiền chưa được tối ưu theo từng lô nguyên liệu, buộc phải giữ tỷ lệ Clinker cao để đảm bảo cường độ R28.",
			SolutionSummary: "Tối ưu Blaine và phụ gia, giảm 3,5% tỷ lệ Clinker.",
			SolutionDetail:  "Thử nghiệm ma trận 12 tổ hợp Blaine/phụ gia trong 30 ngày, hiệu chỉnh PID máy nghiền và kiểm soát cường độ nén R28 theo từng ca.",
			BenefitSummary:  "Giảm 3,5% Clinker, làm lợi ròng 1,22 tỷ VNĐ/năm sau nghiệm thu 30 ngày.",
			Gross:           1_280_000_000, Invest: 60_000_000, Annex03: true, DaysAgo: 4,
			Split:        "70% Trưởng nhóm / 30% KCS",
			Participants: map[string]float64{"CF-0711": 30},
		},
		{
			Track: TrackStandard, Stage: StageScreen, ImplStatus: "IDEA",
			Title: "Hợp nhất dữ liệu Trạm cân – SCADA – CEMS – ERP xóa bỏ nhập liệu kép",
			Dept:  "SUPPORT", Submitter: "CF-0611", Equipment: "ERP-SSOT-2026",
			ProblemSummary:  "Nhập liệu trùng lặp giữa phòng Cân, Kế toán và Sản xuất gây sai lệch.",
			ProblemDetail:   "Ba bộ phận nhập cùng một chứng từ theo ba biểu mẫu khác nhau; sai lệch số liệu mất 160 giờ công/tháng để đối chiếu.",
			SolutionSummary: "Kết nối API đồng bộ phiếu cân, chỉ số điện/nhiệt và hóa đơn vận tải.",
			SolutionDetail:  "Xây dựng Single Source of Truth, chuẩn hóa khóa chứng từ và tự động tính giá thành động theo ca; không thay thế phần mềm ERP hiện hữu.",
			BenefitSummary:  "Tiết kiệm 160 giờ công/tháng, làm lợi ròng 340 triệu VNĐ/năm.",
			Gross:           360_000_000, Invest: 20_000_000, Annex03: true, DaysAgo: 3,
			Split:        "50% Kế toán / 50% MIS",
			Participants: map[string]float64{"CF-0811": 50},
		},
		{
			Track: TrackStandard, Stage: StageImplement, ImplStatus: "IMPLEMENTED",
			Title: "Số hóa kiểm kê khí nhà kính (MRV/CBAM) từ dữ liệu CEMS",
			Dept:  "QHSE", Submitter: "CF-0711", Equipment: "CEMS-MRV-01",
			ProblemSummary:  "Tổng hợp phát thải CO2 thủ công cuối quý tốn nhiều nhân lực.",
			ProblemDetail:   "Dữ liệu CEMS, trạm cân và sản lượng được xuất Excel rồi tổng hợp thủ công, dễ sai sót và chậm báo cáo CBAM.",
			SolutionSummary: "Tự động tính phát thải và tối ưu phun hóa chất NOx/SO2.",
			SolutionDetail:  "Pipeline đọc CEMS mỗi 5 phút, chuẩn hóa theo MRV, xuất báo cáo CBAM và gợi ý định lượng hóa chất theo tải lò.",
			BenefitSummary:  "Giảm nhân công báo cáo và hóa chất, làm lợi ròng 490 triệu VNĐ/năm.",
			Gross:           520_000_000, Invest: 30_000_000, Annex03: true, DaysAgo: 12,
			Split:        "60% Tác giả chính / 40% Tự động hóa",
			Participants: map[string]float64{"CF-0611": 40},
		},
		{
			Track: TrackStandard, Stage: StageApplied, ImplStatus: "IMPLEMENTED",
			Title: "Cổng đặt hàng số & cấp số tự động qua QR cho xe đại lý, trạm trộn B2B",
			Dept:  "SALES", Submitter: "CF-0811", Equipment: "CRM-DEALER-QR",
			ProblemSummary:  "Xe đại lý chờ trung bình 27 phút tại cổng Hải Phòng và Hiệp Phước.",
			ProblemDetail:   "Quy trình đặt hàng và cấp số thủ công qua điện thoại, không liên thông với điều vận và trạm cân.",
			SolutionSummary: "Cổng đặt hàng số, QR cấp số tự động và hàng đợi điều vận.",
			SolutionDetail:  "Tích hợp CRM – điều vận – trạm cân; đại lý tự đặt lịch, nhận QR, xe vào cổng không cần nhập lại thông tin.",
			BenefitSummary:  "Giảm 27 phút chờ/xe, tăng sản lượng xi măng xanh B2B; làm lợi ròng 585 triệu VNĐ/năm.",
			Gross:           640_000_000, Invest: 55_000_000, Annex03: true, DaysAgo: 25,
			Split:        "65% Kinh doanh / 35% Điều vận",
			Participants: map[string]float64{"CF-0911": 35},
		},
		{
			Track: TrackFast48, Stage: StageApplied, ImplStatus: "IMPLEMENTED",
			Title: "Cải tiến che chắn bụi tại điểm chuyển tiếp băng tải số 7",
			Dept:  "KILN", Submitter: "CF-0118", Equipment: "CV-07",
			ProblemSummary:  "Bụi phát tán tại điểm chuyển tiếp gây mất an toàn vệ sinh.",
			ProblemDetail:   "Tấm che cũ hở, bụi phát tán khi băng tải chạy tải cao; công nhân phải vệ sinh mỗi ca.",
			SolutionSummary: "Tự chế tấm che vành khuyên kín kết hợp hút bụi cục bộ.",
			SolutionDetail:  "Gia công tấm che theo biên dạng băng tải, bổ sung hút bụi tận dụng quạt cũ; hoàn thành trong 36 giờ, không cần Capex.",
			BenefitSummary:  "Giảm bụi, tiết kiệm 2 giờ vệ sinh/ngày; làm lợi ròng 18 triệu VNĐ/năm.",
			Gross:           25_000_000, Invest: 7_000_000, Annex03: false, DaysAgo: 18,
			Split: "100% Quản đốc hiện trường",
		},
		{
			Track: TrackStandard, Stage: StageDraft, ImplStatus: "IDEA",
			Title: "Giảm tiêu hao bi nghiền nhờ hiệu chỉnh phân ly gió",
			Dept:  "SUPPORT", Submitter: "CF-0611", Equipment: "MILL-SEP-02",
			ProblemSummary:  "Bi nghiền hao hụt nhanh hơn định mức 8%.",
			ProblemDetail:   "Phân ly gió chưa cân bằng, hạt mịn tuần hoàn làm tăng tải nghiền và mài mòn bi.",
			SolutionSummary: "Cân chỉnh cánh phân ly và bổ sung đo rung định kỳ.",
			SolutionDetail:  "Đang ở bản nháp, cần bổ sung số liệu đo rung và định mức tiêu hao 12 tháng.",
			BenefitSummary:  "Dự kiến làm lợi ròng 45 triệu VNĐ/năm; cần nghiệm thu thêm.",
			Gross:           55_000_000, Invest: 10_000_000, Annex03: false, DaysAgo: 0,
			Split: "100% Tác giả",
		},
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for idx, d := range demos {
		code := fmt.Sprintf("SI-%04d-%05d", year, idx+1)
		now := time.Now().UTC()
		created := now.AddDate(0, 0, -d.DaysAgo).Add(-3 * time.Hour)
		net := NetBenefit(d.Gross, d.Invest)
		reward := ComputeFinancialReward(net, d.Annex03)
		annex, anon := 0, 0
		if d.Annex03 {
			annex = 1
		}
		if d.Track == TrackConfidential {
			anon = 1
		}
		recordStatus := StatusOpen
		rejectReason := sql.NullString{}
		if d.Stage == "REJECTED" {
			recordStatus = StatusRejected
			rejectReason = sql.NullString{String: "Không đáp ứng điều kiện Mục 3.1", Valid: true}
		}
		submitted := created
		due := dueForTrack(d.Track, created)
		submittedSQL := sql.NullString{String: submitted.Format(time.RFC3339), Valid: true}
		dueSQL := sql.NullString{String: due.Format(time.RFC3339), Valid: true}
		if d.Stage == StageDraft {
			submittedSQL = sql.NullString{}
			dueSQL = sql.NullString{}
		}
		paidAt := sql.NullString{}
		paidBy := sql.NullString{}
		if d.Stage == StageApplied {
			paidAt = sql.NullString{String: now.AddDate(0, 0, -1).Format(time.RFC3339), Valid: true}
			paidBy = sql.NullString{String: "CF-0088", Valid: true}
		}
		stageForSQL := d.Stage
		if stageForSQL == "REJECTED" {
			stageForSQL = StageScreen
		}
		res, err := tx.Exec(`
			INSERT INTO initiative(code,track_code,stage_code,record_status,title,department_code,submitter_employee_no,
				equipment_code,problem_summary,problem_detail,solution_summary,solution_detail,benefit_summary,
				implementation_status_code,gross_value_vnd,invest_cost_vnd,net_benefit_vnd,annex03,
				reward_basis,nonfinancial_level,reward_tier_code,base_reward_vnd,reward_amount_vnd,
				seed_granted,anonymous,split_ratio,due_at,submitted_at,paid_at,paid_by,reject_reason,created_by,created_at,updated_at,is_demo)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'FINANCIAL',0,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1)`,
			code, d.Track, stageForSQL, recordStatus, d.Title, d.Dept, d.Submitter,
			d.Equipment, d.ProblemSummary, d.ProblemDetail, d.SolutionSummary, d.SolutionDetail, d.BenefitSummary,
			d.ImplStatus, d.Gross, d.Invest, net, annex,
			reward.TierCode, reward.BaseVND, reward.AmountVND,
			0, anon, d.Split, dueSQL, submittedSQL,
			paidAt, paidBy, rejectReason, d.Submitter, created.Format(time.RFC3339), created.Format(time.RFC3339))
		if err != nil {
			return fmt.Errorf("seed %s: %w", code, err)
		}
		id, _ := res.LastInsertId()
		_, _ = tx.Exec(`INSERT INTO initiative_history(initiative_id,to_stage_code,action_code,changed_by,note,created_at)
			VALUES(?,?,'CREATED',?,'Dữ liệu minh họa',?)`, id, StageDraft, d.Submitter, created.Format(time.RFC3339))
		for _, p := range sortedParticipants(d.Participants) {
			if _, err := tx.Exec(`INSERT INTO initiative_participant(initiative_id,employee_no,share_percent,role_note)
				VALUES(?,?,?,?)`, id, p.no, p.share, "Đồng tác giả"); err != nil {
				return err
			}
		}
		if err := seedHistoryAndReviews(tx, id, d, created, submitted, reward); err != nil {
			return fmt.Errorf("seed history %s: %w", code, err)
		}
	}

	if _, err := tx.Exec(`INSERT INTO initiative_sequence(year,value) VALUES(?,?)
		ON CONFLICT(year) DO UPDATE SET value=excluded.value`, year, len(demos)); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO audit_log(actor,action_code,target,status,detail)
		VALUES('SYSTEM','SEED_DEMO','initiative','200 OK','Nạp dữ liệu minh họa PN-B0000-02')`); err != nil {
		return err
	}
	return tx.Commit()
}

type seedShare struct {
	no    string
	share float64
}

func sortedParticipants(m map[string]float64) []seedShare {
	out := make([]seedShare, 0, len(m))
	for k, v := range m {
		out = append(out, seedShare{k, v})
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].no < out[j-1].no; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func seedHistoryAndReviews(tx *sql.Tx, id int64, d demoInitiative, created, submitted time.Time, reward RewardResult) error {
	type step struct {
		from, to string
		action   string
		actor    string
		note     string
		at       time.Time
	}
	var steps []step
	if d.Stage == StageDraft {
		steps = nil
	} else if d.Track == TrackConfidential {
		steps = []step{{StageDraft, StageReview, "SUBMITTED", d.Submitter, "Chuyển thẳng TGĐ & KTNB", submitted.Add(2 * time.Hour)}}
	} else {
		steps = []step{{StageDraft, StageScreen, "SUBMITTED", d.Submitter, "Nộp hồ sơ", submitted}}
		if d.Stage != StageScreen {
			steps = append(steps, step{StageScreen, StageReview, "SCREENED", "CF-0118", "Xác nhận hiện trường", submitted.Add(6 * time.Hour)})
		}
		if d.Stage != StageScreen && d.Stage != StageReview {
			steps = append(steps, step{StageReview, StageApprove, "REVIEWED", "CF-0010", "Hội đồng thẩm định Net Benefit", submitted.Add(24 * time.Hour)})
		}
		if d.Stage == StageImplement || d.Stage == StageApplied {
			steps = append(steps, step{StageApprove, StageImplement, "APPROVED", "CF-0001", "Phê duyệt quyết định thưởng", submitted.Add(48 * time.Hour)})
		}
		if d.Stage == StageApplied {
			steps = append(steps, step{StageImplement, StageApplied, "PAID", "CF-0088", "Chi trả cùng kỳ lương", submitted.Add(72 * time.Hour)})
		}
	}
	for _, s := range steps {
		_, _ = tx.Exec(`INSERT INTO initiative_history(initiative_id,from_stage_code,to_stage_code,action_code,changed_by,note,created_at)
			VALUES(?,?,?,?,?,?,?)`, id, s.from, s.to, s.action, s.actor, s.note, s.at.Format(time.RFC3339))
	}

	if d.Track == TrackConfidential {
		if d.Stage != StageReview {
			_, _ = tx.Exec(`INSERT INTO initiative_review(initiative_id,stage_code,reviewer_employee_no,decision,comment,
				assessed_gross_vnd,assessed_invest_vnd,assessed_net_vnd,reward_tier_code,reward_amount_vnd,created_at)
				VALUES(?,?,?,'APPROVE',?,?,?,?,?,?,?)`, id, StageReview, "CF-0001", "TGĐ & KTNB xác nhận bằng chứng",
				d.Gross, d.Invest, NetBenefit(d.Gross, d.Invest), reward.TierCode, reward.AmountVND, submitted.Add(4*time.Hour).Format(time.RFC3339))
		}
		return nil
	}

	if d.Stage != StageScreen {
		_, _ = tx.Exec(`INSERT INTO initiative_review(initiative_id,stage_code,reviewer_employee_no,decision,comment,created_at)
			VALUES(?,?,?,'APPROVE',?,?)`, id, StageScreen, "CF-0118",
			"Đã kiểm tra hiện trường và xác nhận số liệu vận hành.", submitted.Add(5*time.Hour).Format(time.RFC3339))
	}
	if d.Stage == StageApprove || d.Stage == StageImplement || d.Stage == StageApplied {
		_, _ = tx.Exec(`INSERT INTO initiative_review(initiative_id,stage_code,reviewer_employee_no,decision,comment,
			assessed_gross_vnd,assessed_invest_vnd,assessed_net_vnd,reward_tier_code,reward_amount_vnd,created_at)
			VALUES(?,?,?,'APPROVE',?,?,?,?,?,?,?)`, id, StageReview, "CF-0010",
			"Hội đồng đối chiếu SCADA/CEMS/trạm cân và thống nhất mức thưởng.",
			d.Gross, d.Invest, NetBenefit(d.Gross, d.Invest), reward.TierCode, reward.AmountVND,
			submitted.Add(23*time.Hour).Format(time.RFC3339))
	}
	return nil
}
