package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// AwardContributionShare credits the contributor of a contributed channel with
// a percentage of the caller's settled charge. It runs exactly once per request
// (relayInfo.ContributionShareAwarded), never for the contributor's own calls,
// and only when the routed model is one this contributor actually provided.
//
// The payout is best-effort by design: a ledger write failure logs loudly but
// must never fail (or retry) the caller's request, which has already settled.
func AwardContributionShare(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, actualQuota int) {
	if relayInfo == nil || actualQuota <= 0 {
		return
	}
	if relayInfo.ContributionShareAwarded {
		return
	}
	if relayInfo.ChannelMeta == nil || relayInfo.ChannelMeta.ContributionId <= 0 {
		return
	}
	if relayInfo.UserId <= 0 {
		return
	}

	contribution, err := model.GetContributionById(relayInfo.ChannelMeta.ContributionId)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("contribution share: contribution %d not found: %v", relayInfo.ChannelMeta.ContributionId, err))
		return
	}
	contributorUserId := contribution.UserId
	// Self-calls by the contributor are excluded: they would otherwise let a
	// contributor spend (1 - share%) of the model price and re-mint the rest.
	if contributorUserId <= 0 || relayInfo.UserId == contributorUserId {
		return
	}

	// Only models the contributor actually submitted for this channel qualify.
	approvedModels, err := model.GetApprovedContributionModelsForChannel(contribution.Id)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("contribution share: load approved models failed: %v", err))
		return
	}
	contributed, ok := approvedModels[relayInfo.OriginModelName]
	if !ok || contributed.ChannelId != relayInfo.ChannelId || contributed.ChannelId == 0 {
		return
	}

	percent := contributionPercent(contribution)
	if percent <= 0 {
		return
	}

	// The share is a bounded percentage of a bounded quota. Conversion goes
	// through the centralized saturated helpers; a clamp (e.g. an extreme
	// percentage/huge quota) is audited instead of silently truncating.
	shareQuota, clamp := common.QuotaFromFloatChecked(float64(actualQuota) * float64(percent) / 100)
	if shareQuota <= 0 {
		return
	}
	if clamp != nil {
		logger.LogWarn(ctx, fmt.Sprintf("contribution share saturated: op=%s kind=%s original=%g clamped=%d contributor=%d model=%s",
			clamp.Op, clamp.Kind, clamp.Original, clamp.Clamped, contributorUserId, relayInfo.OriginModelName))
	}

	if err := model.IncreaseContributionQuota(contributorUserId, shareQuota, false); err != nil {
		logger.LogError(ctx, fmt.Sprintf("failed to credit contribution share: user=%d quota=%d err=%v",
			contributorUserId, shareQuota, err))
		return
	}
	if err := model.RecordContributionEarning(&model.ContributionEarning{
		ContributorUserId: contributorUserId,
		CallerUserId:      relayInfo.UserId,
		ContributionId:    contribution.Id,
		ChannelId:         relayInfo.ChannelId,
		ModelName:         relayInfo.OriginModelName,
		UpstreamModel:     contributed.UpstreamModel,
		RequestId:         relayInfo.RequestId,
		CallerQuota:       actualQuota,
		SharePercent:      percent,
		ShareQuota:        shareQuota,
	}); err != nil {
		// The balance credit succeeded but the ledger write failed: the two
		// balance pages would disagree. Log loudly so it can be reconciled
		// against the consume log via RequestId.
		logger.LogError(ctx, fmt.Sprintf("contribution share credited but ledger write failed: user=%d share=%d request=%s err=%v",
			contributorUserId, shareQuota, relayInfo.RequestId, err))
	}

	// Idempotency: mark the award before returning so a repeated settle on the
	// same relayInfo (retries, explicit settle paths) never credits twice.
	relayInfo.ContributionShareAwarded = true

	if clamp != nil {
		// Surface the saturation marker on this request's consume log if one
		// exists, otherwise it is only in the backend logs generated above.
		logger.LogWarn(ctx, fmt.Sprintf("contribution share clamp without consume log: %+v", clamp))
	}
}

// contributionPercent resolves the effective share percentage: the contributor
// override wins, otherwise the global default.
func contributionPercent(contribution *model.ChannelContribution) int {
	if contribution == nil {
		return 0
	}
	if contribution.SharePercent != nil {
		return *contribution.SharePercent
	}
	user, err := model.GetUserById(contribution.UserId, false)
	if err == nil && user.ContributionSharePercent != nil {
		return *user.ContributionSharePercent
	}
	return operation_setting.GetContributionSetting().DefaultSharePercent
}
