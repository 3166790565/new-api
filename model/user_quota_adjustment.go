package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

var (
	ErrInvalidUserQuotaAdjustment = errors.New("invalid user quota adjustment")
	ErrUserQuotaPermission        = errors.New("cannot adjust quota for this user role")
)

// UserQuotaAdjustment is the immutable database snapshot of a committed manual
// adjustment. Pending relay deductions in the quota cache are not part of it.
type UserQuotaAdjustment struct {
	UserID   int
	Username string
	Before   int
	After    int
}

func AdjustUserQuota(userID, operatorRole int, mode string, value int) (*UserQuotaAdjustment, error) {
	if userID <= 0 || (mode != "add" && mode != "subtract" && mode != "override") {
		return nil, ErrInvalidUserQuotaAdjustment
	}
	if mode != "override" && value <= 0 {
		return nil, ErrInvalidUserQuotaAdjustment
	}
	if value > common.MaxWalletQuota || value < -common.MaxWalletQuota {
		return nil, ErrWalletQuotaLimitExceeded
	}

	var adjustment UserQuotaAdjustment
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).First(&user, userID).Error; err != nil {
			return err
		}
		if operatorRole != common.RoleRootUser && operatorRole <= user.Role {
			return ErrUserQuotaPermission
		}
		if user.Quota > common.MaxWalletQuota || user.Quota < -common.MaxWalletQuota {
			return ErrWalletQuotaLimitExceeded
		}
		quota := decimal.NewFromInt(int64(value))
		switch mode {
		case "add":
			quota = decimal.NewFromInt(int64(user.Quota)).Add(quota)
		case "subtract":
			quota = decimal.NewFromInt(int64(user.Quota)).Sub(quota)
		}
		after, err := common.WalletQuotaFromDecimalStrict(quota)
		if err != nil {
			return ErrWalletQuotaLimitExceeded
		}
		// An unchanged override is a successful operation, including on MySQL
		// configurations that count only changed rows in RowsAffected.
		if after != user.Quota {
			result := tx.Model(&User{}).Where("id = ?", userID).Update("quota", after)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
		}
		adjustment = UserQuotaAdjustment{UserID: user.Id, Username: user.Username, Before: user.Quota, After: after}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Apply only the committed difference, preserving outstanding reservations.
	// Both balances are bounded above, so their difference fits in int64.
	delta := int64(adjustment.After) - int64(adjustment.Before)
	if delta != 0 {
		if err := cacheIncrUserQuota(userID, delta); err != nil {
			common.SysError(fmt.Sprintf("failed to sync manual quota adjustment for user %d: %s", userID, err))
		}
	}
	return &adjustment, nil
}

// IncreaseContributionQuota credits a contributor's share balance. It mirrors
// IncreaseUserQuota: the update is bounded by MaxWalletQuota in the WHERE clause
// so a saturated credit cannot wrap, and the committed delta is applied to the
// user cache so the balance is immediately visible without waiting for the hash
// to expire.
func IncreaseContributionQuota(userId int, quota int, db bool) error {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	if quota == 0 {
		return nil
	}
	if err := common.ValidateWalletQuota(quota); err != nil {
		return err
	}
	if !db && common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeUserContributionQuota, userId, quota)
		return nil
	}
	return increaseContributionQuota(userId, quota)
}

func increaseContributionQuota(userId int, quota int) error {
	result := DB.Model(&User{}).
		Where("id = ? AND contribution_quota <= ?", userId, common.MaxWalletQuota-quota).
		Update("contribution_quota", gorm.Expr("contribution_quota + ?", quota))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrWalletQuotaLimitExceeded
	}
	gopool.Go(func() {
		if err := cacheIncrUserContributionQuota(userId, int64(quota)); err != nil {
			common.SysLog("failed to increase user contribution quota: " + err.Error())
		}
	})
	return nil
}

// DecreaseContributionQuota debits a contributor's share balance only when it
// covers the requested amount, so a transfer can never overdraw it.
func DecreaseContributionQuota(userId int, quota int) error {
	if quota <= 0 {
		return errors.New("quota must be positive")
	}
	result := DB.Model(&User{}).
		Where("id = ? AND contribution_quota >= ?", userId, quota).
		Update("contribution_quota", gorm.Expr("contribution_quota - ?", quota))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("贡献余额不足")
	}
	gopool.Go(func() {
		if err := cacheIncrUserContributionQuota(userId, -int64(quota)); err != nil {
			common.SysLog("failed to decrease user contribution quota: " + err.Error())
		}
	})
	return nil
}

// TransferContributionQuotaToQuota moves contribution earnings into the user's
// spendable wallet quota. Contributions are already bounded by MaxWalletQuota,
// so the sum still fits the wallet bound; the wallet side is checked against it
// explicitly.
func (user *User) TransferContributionQuotaToQuota(quota int) error {
	if quota <= 0 {
		return errors.New("转移额度必须大于 0！")
	}
	if err := common.ValidateWalletQuota(quota); err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(user, user.Id).Error; err != nil {
			return err
		}
		if user.ContributionQuota < quota {
			return errors.New("贡献余额不足！")
		}
		if err := common.ValidateWalletQuota(user.Quota + quota); err != nil {
			return err
		}
		user.ContributionQuota -= quota
		user.Quota += quota
		if err := tx.Save(user).Error; err != nil {
			return err
		}
		return nil
	})
}
