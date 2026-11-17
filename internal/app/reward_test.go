package app

import "testing"

func TestComputeFinancialRewardTiers(t *testing.T) {
	cases := []struct {
		net      int64
		annex    bool
		wantTier string
		want     int64
	}{
		{-1, false, "T1_IDEA", 300_000},
		{0, false, "T1_IDEA", 300_000},
		{0, true, "T1_IDEA", 360_000},
		{19_999_999, false, "T2", 1_000_000},
		{20_000_000, false, "T3", 2_500_000},
		{49_999_999, false, "T3", 2_500_000},
		{50_000_000, false, "T4", 5_000_000},
		{149_999_999, false, "T4", 5_000_000},
		{150_000_000, false, "T5", 10_000_000},
		{299_999_999, false, "T5", 10_000_000},
		{300_000_000, false, "T6", 20_000_000},
		{499_999_999, false, "T6", 20_000_000},
		{500_000_000, false, "T7", 30_000_000},
		{1_000_000_000, false, "T7", 60_000_000},
		{2_000_000_000, false, "T7", 100_000_000},
		{1_000_000_000, true, "T7", 72_000_000},
		{300_000_000, true, "T6", 24_000_000},
	}
	for _, c := range cases {
		got := ComputeFinancialReward(c.net, c.annex)
		if got.TierCode != c.wantTier || got.AmountVND != c.want {
			t.Errorf("net=%d annex=%v: got %s/%d, want %s/%d", c.net, c.annex, got.TierCode, got.AmountVND, c.wantTier, c.want)
		}
	}
}

func TestComputeNonFinancialReward(t *testing.T) {
	cases := map[int]int64{1: 1_000_000, 2: 3_000_000, 3: 8_000_000, 4: 25_000_000}
	for level, want := range cases {
		got := NonFinancialReward(level)
		if got.AmountVND != want || got.TierCode == "" {
			t.Errorf("level %d: got %+v", level, got)
		}
	}
	if got := NonFinancialReward(0); got.AmountVND != 0 {
		t.Errorf("level 0 should be zero, got %+v", got)
	}
	if got := ComputeReward(1_000_000, true, BasisNonFinancial, 2); got.AmountVND != 3_000_000 {
		t.Errorf("non-financial must not receive annex multiplier: %+v", got)
	}
}

func TestNetBenefitClampsAtZero(t *testing.T) {
	if got := NetBenefit(10, 20); got != 0 {
		t.Fatalf("got %d", got)
	}
	if got := NetBenefit(100, 25); got != 75 {
		t.Fatalf("got %d", got)
	}
}
