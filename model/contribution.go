package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// ChannelContribution is one user-submitted upstream channel awaiting or
// having received an administrator decision. Its models live in
// ChannelContributionModel, one row per contributed model, because an
// administrator may approve or reject each model independently.
type ChannelContribution struct {
	Id              int     `json:"id"`
	UserId          int     `json:"user_id" gorm:"index;not null"`
	Type            int     `json:"type" gorm:"default:0"`
	Name            string  `json:"name" gorm:"index"`
	BaseURL         *string `json:"base_url" gorm:"column:base_url;default:''"`
	Key             string  `json:"key" gorm:"not null"`
	Group           string  `json:"group" gorm:"type:varchar(64);default:'default'"`
	Priority        *int64  `json:"priority" gorm:"bigint;default:0"`
	Weight          *uint   `json:"weight" gorm:"default:0"`
	Status          int     `json:"status" gorm:"index;default:0"`
	RejectReason    string  `json:"reject_reason" gorm:"type:varchar(255)"`
	SharePercent    *int    `json:"share_percent"` // nil继承全局默认比例
	SourceChannelId int     `json:"source_channel_id" gorm:"index;default:0"`
	TestModel       *string `json:"test_model"`
	TestTime        int64   `json:"test_time" gorm:"bigint;default:0"`
	ReviewerId      int     `json:"reviewer_id" gorm:"index;default:0"`
	ReviewedTime    int64   `json:"reviewed_time" gorm:"bigint;default:0"`
	CreatedTime     int64   `json:"created_time" gorm:"bigint"`
}

const (
	ContributionStatusPending           = 0
	ContributionStatusApproved          = 1
	ContributionStatusPartiallyApproved = 2
	ContributionStatusRejected          = 3
	ContributionStatusWithdrawn         = 4
	ContributionStatusSuperseded        = 5
)

// ChannelContributionModel is one contributed model and its review outcome.
// After approval ChannelId points at the concrete Channel created from this
// row, which is the only relation the relay hot path relies on.
type ChannelContributionModel struct {
	Id             int    `json:"id"`
	ContributionId int    `json:"contribution_id" gorm:"index;not null"`
	UpstreamModel  string `json:"upstream_model" gorm:"type:varchar(255);not null"`
	PublicModel    string `json:"public_model" gorm:"type:varchar(255);not null"`
	RejectReason   string `json:"reject_reason" gorm:"type:varchar(255)"`
	ChannelId      int    `json:"channel_id" gorm:"index;default:0"`
	CreatedTime    int64  `json:"created_time" gorm:"bigint"`
}

const (
	ContributionModelStatusPending  = 0
	ContributionModelStatusApproved = 1
	ContributionModelStatusRejected = 2
)

// ContributionEarning is the per-request share ledger. One row per settled
// request that routed through a contributed model; RequestId correlates it
// with the caller's consume log for reconciliation.
type ContributionEarning struct {
	Id                int    `json:"id"`
	ContributorUserId int    `json:"contributor_user_id" gorm:"index;not null"`
	CallerUserId      int    `json:"caller_user_id" gorm:"index;default:0"`
	ContributionId    int    `json:"contribution_id" gorm:"index;default:0"`
	ChannelId         int    `json:"channel_id" gorm:"index;default:0"`
	ModelName         string `json:"model_name" gorm:"type:varchar(255)"`
	UpstreamModel     string `json:"upstream_model" gorm:"type:varchar(255)"`
	RequestId         string `json:"request_id" gorm:"type:varchar(64);index"`
	CallerQuota       int    `json:"caller_quota"`
	SharePercent      int    `json:"share_percent"`
	ShareQuota        int    `json:"share_quota"`
	CreatedTime       int64  `json:"created_time" gorm:"bigint"`
}

var (
	ErrContributionNotFound = errors.New("contribution not found")
	ErrContributionState    = errors.New("contribution state does not allow this operation")
)

func (ChannelContribution) TableName() string {
	return "channel_contributions"
}

func (ChannelContributionModel) TableName() string {
	return "channel_contribution_models"
}

func (ContributionEarning) TableName() string {
	return "contribution_earnings"
}

func (c *ChannelContribution) GetBaseURL() string {
	if c.BaseURL == nil {
		return ""
	}
	return *c.BaseURL
}

func (c *ChannelContribution) GetPriority() int64 {
	if c.Priority == nil {
		return 0
	}
	return *c.Priority
}

func (c *ChannelContribution) GetWeight() int {
	if c.Weight == nil {
		return 0
	}
	return int(*c.Weight)
}

func InsertContributionWithModels(contribution *ChannelContribution, models []ChannelContributionModel) error {
	if contribution == nil || len(models) == 0 {
		return errors.New("contribution and at least one model are required")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		contribution.CreatedTime = common.GetTimestamp()
		if err := tx.Create(contribution).Error; err != nil {
			return err
		}
		for i := range models {
			models[i].ContributionId = contribution.Id
			models[i].CreatedTime = contribution.CreatedTime
		}
		return tx.Create(&models).Error
	})
}

func GetContributionById(id int) (*ChannelContribution, error) {
	if id <= 0 {
		return nil, ErrContributionNotFound
	}
	var contribution ChannelContribution
	if err := DB.First(&contribution, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrContributionNotFound
		}
		return nil, err
	}
	return &contribution, nil
}

func GetContributionModels(contributionId int) ([]ChannelContributionModel, error) {
	var models []ChannelContributionModel
	err := DB.Where("contribution_id = ?", contributionId).Order("id asc").Find(&models).Error
	return models, err
}

// ReplaceContributionModels rewrites the contributed model list of a pending
// contribution. Only pending submissions may be edited.
func ReplaceContributionModels(contributionId int, models []ChannelContributionModel) error {
	if len(models) == 0 {
		return errors.New("at least one model is required")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var contribution ChannelContribution
		if err := lockForUpdate(tx).First(&contribution, contributionId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrContributionNotFound
			}
			return err
		}
		if contribution.Status != ContributionStatusPending {
			return ErrContributionState
		}
		if err := tx.Where("contribution_id = ?", contributionId).Delete(&ChannelContributionModel{}).Error; err != nil {
			return err
		}
		now := common.GetTimestamp()
		for i := range models {
			models[i].Id = 0
			models[i].ContributionId = contributionId
			models[i].CreatedTime = now
		}
		return tx.Create(&models).Error
	})
}

func ListContributionsByUser(userId int, offset int, limit int) ([]ChannelContribution, int64, error) {
	var total int64
	query := DB.Model(&ChannelContribution{}).Where("user_id = ?", userId)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var contributions []ChannelContribution
	err := query.Order("id desc").Limit(limit).Offset(offset).Find(&contributions).Error
	return contributions, total, err
}

func ListContributionsByStatus(status int, offset int, limit int) ([]ChannelContribution, int64, error) {
	var total int64
	query := DB.Model(&ChannelContribution{}).Where("status = ?", status)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var contributions []ChannelContribution
	err := query.Order("id asc").Limit(limit).Offset(offset).Find(&contributions).Error
	return contributions, total, err
}

func ContributionStatusCounts() (map[int]int64, error) {
	type row struct {
		Status int
		Count  int64
	}
	var rows []row
	if err := DB.Model(&ChannelContribution{}).Select("status, count(*) as count").Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[int]int64, len(rows))
	for _, r := range rows {
		counts[r.Status] = r.Count
	}
	return counts, nil
}

// WithdrawContribution lets the owner cancel a submission that has not been
// reviewed yet.
func WithdrawContribution(contributionId, userId int) error {
	result := DB.Model(&ChannelContribution{}).
		Where("id = ? AND user_id = ? AND status = ?", contributionId, userId, ContributionStatusPending).
		Updates(map[string]any{"status": ContributionStatusWithdrawn, "reviewed_time": common.GetTimestamp()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrContributionState
	}
	return nil
}

// UpdatePendingContributionDetails updates the connection fields of a pending
// submission. Approved submissions are immutable: changing them requires a new
// submission so the credential change is reviewed again.
func UpdatePendingContributionDetails(contributionId, userId int, update ChannelContribution) error {
	result := DB.Model(&ChannelContribution{}).
		Where("id = ? AND user_id = ? AND status = ?", contributionId, userId, ContributionStatusPending).
		Updates(map[string]any{
			"type":         update.Type,
			"name":         update.Name,
			"base_url":     update.BaseURL,
			"key":          update.Key,
			"group":        update.Group,
			"priority":     update.Priority,
			"weight":       update.Weight,
			"test_model":   update.TestModel,
			"test_time":    update.TestTime,
			"reject_reason": "",
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrContributionState
	}
	return nil
}

// ListContributors returns the users who own at least one approved
// contribution, together with their aggregate contribution counts.
type ContributionContributor struct {
	UserId          int    `json:"user_id"`
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	TotalCount      int64  `json:"total_count"`
	ApprovedCount   int64  `json:"approved_count"`
	PendingCount    int64  `json:"pending_count"`
	ContributionQuota int  `json:"contribution_quota"`
	SharePercent    *int   `json:"share_percent"`
}

// ListContributors joins contributions with users so the admin overview can
// show a username without a per-row lookup.
func ListContributors(offset int, limit int) ([]ContributionContributor, int64, error) {
	type aggregate struct {
		UserId        int
		TotalCount    int64
		ApprovedCount int64
		PendingCount  int64
	}
	var aggregates []aggregate
	err := DB.Model(&ChannelContribution{}).
		Select("user_id, count(*) as total_count, "+
			"sum(case when status = ? then 1 else 0 end) as approved_count, "+
			"sum(case when status = ? then 1 else 0 end) as pending_count",
			ContributionStatusApproved, ContributionStatusPending).
		Group("user_id").
		Order("user_id asc").
		Scan(&aggregates).Error
	if err != nil {
		return nil, 0, err
	}
	total := int64(len(aggregates))
	start := min(offset, len(aggregates))
	end := min(start+limit, len(aggregates))
	page := aggregates[start:end]
	contributors := make([]ContributionContributor, 0, len(page))
	for _, item := range page {
		user, err := GetUserById(item.UserId, false)
		if err != nil {
			continue
		}
		contributors = append(contributors, ContributionContributor{
			UserId:            item.UserId,
			Username:          user.Username,
			DisplayName:       user.DisplayName,
			TotalCount:        item.TotalCount,
			ApprovedCount:     item.ApprovedCount,
			PendingCount:      item.PendingCount,
			ContributionQuota: user.ContributionQuota,
			SharePercent:      user.ContributionSharePercent,
		})
	}
	return contributors, total, nil
}

func GetContributionEarnings(contributorUserId int, offset int, limit int) ([]ContributionEarning, int64, error) {
	var total int64
	query := DB.Model(&ContributionEarning{}).Where("contributor_user_id = ?", contributorUserId)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var earnings []ContributionEarning
	err := query.Order("id desc").Limit(limit).Offset(offset).Find(&earnings).Error
	return earnings, total, err
}

func RecordContributionEarning(earning *ContributionEarning) error {
	if earning == nil {
		return errors.New("earning is required")
	}
	earning.CreatedTime = common.GetTimestamp()
	return DB.Create(earning).Error
}

// SumContributionEarnings reports the lifetime share credited to a contributor.
func SumContributionEarnings(contributorUserId int) (int64, error) {
	var sumSum int64
	err := DB.Model(&ContributionEarning{}).
		Where("contributor_user_id = ?", contributorUserId).
		Select("COALESCE(SUM(share_quota), 0)").
		Scan(&sumSum).Error
	return sumSum, err
}

// GetApprovedContributionModelsForChannel returns the approved contributed
// models materialized as the given channel, keyed by the public (routing)
// model name. The relay share path needs it to decide whether a request's
// model is actually one this contributor provided on that channel.
func GetApprovedContributionModelsForChannel(contributionId int) (map[string]ChannelContributionModel, error) {
	if contributionId <= 0 {
		return nil, nil
	}
	var models []ChannelContributionModel
	err := DB.Where("contribution_id = ? AND status = ?", contributionId, ContributionModelStatusApproved).
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]ChannelContributionModel, len(models))
	for _, item := range models {
		result[item.PublicModel] = item
	}
	return result, nil
}

func FormatContributionKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "***"
	}
	return fmt.Sprintf("%s***%s", key[:4], key[len(key)-4:])
}
