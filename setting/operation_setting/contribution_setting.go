package operation_setting

import (
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/setting/config"
)

// ContributionSetting backs the user-contributed channel feature. The global
// default share percent applies to contributors without a per-user override;
// a value of 0 disables payouts for that contributor.
type ContributionSetting struct {
	Enabled             bool `json:"enabled"`               // 是否启用贡献渠道功能
	DefaultSharePercent int  `json:"default_share_percent"` // 全局默认分成百分比（0-100）
	MaxPendingPerUser   int  `json:"max_pending_per_user"`  // 每用户最多待审提交数，0 表示不限制
}

var contributionSetting = ContributionSetting{
	Enabled:             false,
	DefaultSharePercent: 10,
	MaxPendingPerUser:   5,
}

func init() {
	config.GlobalConfig.Register("contribution_setting", &contributionSetting)
}

func GetContributionSetting() *ContributionSetting {
	return &contributionSetting
}

// NormalizeSharePercent bounds an administrator-supplied percentage so a
// misconfiguration can neither overpay nor create a negative share.
func NormalizeSharePercent(percent int) (int, error) {
	if percent < 0 || percent > 100 {
		return 0, fmt.Errorf("share percent must be between 0 and 100")
	}
	return percent, nil
}

// ValidateContributionOption rejects out-of-range values before they are
// written to the options table.
func ValidateContributionOption(key, value string) error {
	switch key {
	case "contribution_setting.default_share_percent":
		number, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s must be an integer", key)
		}
		_, err = NormalizeSharePercent(number)
		return err
	case "contribution_setting.max_pending_per_user":
		number, err := strconv.Atoi(value)
		if err != nil || number < 0 {
			return fmt.Errorf("%s must be a non-negative integer", key)
		}
	}
	return nil
}
