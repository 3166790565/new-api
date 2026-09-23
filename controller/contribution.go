package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// contributionAllowedChannelTypes restricts contributed channels to the three
// providers whose upstream /models fetch and per-model test are supported by
// the contribution wizard: OpenAI, Anthropic, Gemini.
var contributionAllowedChannelTypes = map[int]struct{}{
	constant.ChannelTypeOpenAI:    {},
	constant.ChannelTypeAnthropic: {},
	constant.ChannelTypeGemini:    {},
}

// isContributionAllowedChannelType reports whether a channel type may be
// contributed.
func isContributionAllowedChannelType(channelType int) bool {
	_, ok := contributionAllowedChannelTypes[channelType]
	return ok
}

// validateContributionBaseURL rejects a base URL that ends with a trailing
// slash. Callers must pass a provider base URL without a model-specific path,
// so a trailing slash is always a mistake. An empty base URL is allowed (the
// provider default is used instead).
func validateContributionBaseURL(baseURL string) error {
	if strings.HasSuffix(baseURL, "/") {
		return errors.New("API 地址不能以斜杠结尾")
	}
	return nil
}

// buildContributionPreviewChannel builds a throwaway (unsaved) channel for the
// fetch-models / test-model previews. It enforces the allowed type set and the
// base-URL rule, defaulting a blank base URL to the provider default, and keeps
// only the first line of the key (contributed channels are single-key).
func buildContributionPreviewChannel(channelType int, baseURL, key string) (*model.Channel, error) {
	if !isContributionAllowedChannelType(channelType) {
		return nil, errors.New("不支持的渠道类型")
	}
	baseURL = strings.TrimSpace(baseURL)
	if err := validateContributionBaseURL(baseURL); err != nil {
		return nil, err
	}
	if baseURL == "" {
		baseURL = constant.GetChannelBaseURL(channelType)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("密钥不能为空")
	}
	if idx := strings.IndexByte(key, '\n'); idx >= 0 {
		key = strings.TrimSpace(key[:idx])
	}
	return &model.Channel{
		Type:    channelType,
		Key:     key,
		BaseURL: &baseURL,
	}, nil
}

// contributionModelInput is one contributed model in a submit/update payload.
type contributionModelInput struct {
	UpstreamModel string `json:"upstream_model"`
	PublicModel   string `json:"public_model"`
}

// submitContributionRequest is the body for creating or editing a contribution.
type submitContributionRequest struct {
	Type      int                      `json:"type"`
	Name      string                   `json:"name"`
	BaseURL   string                   `json:"base_url"`
	Key       string                   `json:"key"`
	Group     string                   `json:"group"`
	Priority  int64                    `json:"priority"`
	Weight    uint                     `json:"weight"`
	TestModel string                   `json:"test_model"`
	Models    []contributionModelInput `json:"models"`
}

// contributionDetailView wraps a contribution with its per-model rows. The
// embedded Key is masked by callers before the view is returned.
type contributionDetailView struct {
	model.ChannelContribution
	Models []model.ChannelContributionModel `json:"models"`
}

//__CONTRIB_CTRL_BUILDER__

// buildContributionFromRequest validates a submit/update payload and turns it
// into a pending contribution plus its model rows. It normalizes the group,
// defaults a blank public model to the upstream name, and rejects duplicate
// public names (they would collide as channel routing entries).
func buildContributionFromRequest(req submitContributionRequest) (*model.ChannelContribution, []model.ChannelContributionModel, error) {
	if !isContributionAllowedChannelType(req.Type) {
		return nil, nil, errors.New("不支持的渠道类型")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, nil, errors.New("渠道名称不能为空")
	}
	key := strings.TrimSpace(req.Key)
	if key == "" {
		return nil, nil, errors.New("密钥不能为空")
	}
	if err := validateContributionBaseURL(strings.TrimSpace(req.BaseURL)); err != nil {
		return nil, nil, err
	}
	if len(req.Models) == 0 {
		return nil, nil, errors.New("至少需要提交一个模型")
	}
	group := strings.TrimSpace(req.Group)
	if group == "" {
		group = "default"
	}
	models := make([]model.ChannelContributionModel, 0, len(req.Models))
	seen := make(map[string]struct{}, len(req.Models))
	for _, m := range req.Models {
		upstream := strings.TrimSpace(m.UpstreamModel)
		if upstream == "" {
			return nil, nil, errors.New("上游模型名称不能为空")
		}
		public := strings.TrimSpace(m.PublicModel)
		if public == "" {
			public = upstream
		}
		if _, dup := seen[public]; dup {
			return nil, nil, fmt.Errorf("公开模型名称重复：%s", public)
		}
		seen[public] = struct{}{}
		models = append(models, model.ChannelContributionModel{
			UpstreamModel: upstream,
			PublicModel:   public,
			Status:        model.ContributionModelStatusPending,
		})
	}
	baseURL := strings.TrimSpace(req.BaseURL)
	weight := req.Weight
	priority := req.Priority
	contribution := &model.ChannelContribution{
		Type:     req.Type,
		Name:     name,
		BaseURL:  &baseURL,
		Key:      key,
		Group:    group,
		Priority: &priority,
		Weight:   &weight,
		Status:   model.ContributionStatusPending,
	}
	if testModel := strings.TrimSpace(req.TestModel); testModel != "" {
		contribution.TestModel = &testModel
	}
	return contribution, models, nil
}

//__CONTRIB_CTRL_USER__

// fetchContributionModelsRequest is the body for pulling the upstream model list
// from a provider before the channel is submitted. The key is the raw key the
// contributor typed into the wizard.
type fetchContributionModelsRequest struct {
	Type    int    `json:"type"`
	BaseURL string `json:"base_url"`
	Key     string `json:"key"`
}

// FetchContributionModels handles POST /api/contribution/fetch-models — it calls
// the provider's /models endpoint with the contributor's own connection details
// (built as a throwaway channel) and returns the discovered model IDs so the
// wizard can present them for selection. No channel is persisted.
func FetchContributionModels(c *gin.Context) {
	setting := operation_setting.GetContributionSetting()
	if !setting.Enabled {
		common.ApiErrorMsg(c, "渠道贡献功能未开启")
		return
	}
	if c.GetInt("id") <= 0 {
		common.ApiErrorMsg(c, "无效的用户")
		return
	}
	var req fetchContributionModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	channel, err := buildContributionPreviewChannel(req.Type, req.BaseURL, req.Key)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	models, err := fetchChannelUpstreamModelIDs(channel)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("获取模型列表失败: %s", err.Error()),
		})
		return
	}
	common.ApiSuccess(c, gin.H{"data": models})
}

// testContributionModelRequest is the body for testing one contributed model
// against the upstream before submission.
type testContributionModelRequest struct {
	Type         int    `json:"type"`
	BaseURL      string `json:"base_url"`
	Key          string `json:"key"`
	Model        string `json:"model"`
	EndpointType string `json:"endpoint_type"`
	Stream       bool   `json:"stream"`
}

// TestContributionModel handles POST /api/contribution/test-model — it runs a
// single live upstream call for one model using the contributor's connection
// details and reports success plus latency, mirroring the admin channel test
// response shape ({success, message, time}). No channel is persisted.
func TestContributionModel(c *gin.Context) {
	setting := operation_setting.GetContributionSetting()
	if !setting.Enabled {
		common.ApiErrorMsg(c, "渠道贡献功能未开启")
		return
	}
	var req testContributionModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	testModel := strings.TrimSpace(req.Model)
	if testModel == "" {
		common.ApiErrorMsg(c, "测试模型不能为空")
		return
	}
	channel, err := buildContributionPreviewChannel(req.Type, req.BaseURL, req.Key)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	testUserID, err := resolveChannelTestUserID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	requestCtx := context.Background()
	if c.Request != nil {
		requestCtx = c.Request.Context()
	}
	tik := time.Now()
	result := testChannel(requestCtx, channel, testUserID, testModel, req.EndpointType, req.Stream)
	if result.localErr != nil {
		resp := gin.H{
			"success": false,
			"message": result.localErr.Error(),
			"time":    0.0,
		}
		if result.newAPIError != nil {
			resp["error_code"] = result.newAPIError.GetErrorCode()
		}
		c.JSON(http.StatusOK, resp)
		return
	}
	consumedTime := float64(time.Since(tik).Milliseconds()) / 1000.0
	if result.newAPIError != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":    false,
			"message":    result.newAPIError.Error(),
			"time":       consumedTime,
			"error_code": result.newAPIError.GetErrorCode(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"time":    consumedTime,
	})
}

// SubmitContribution handles POST /api/contribution/ — a user offers an upstream
// channel for review. Gated by the global toggle and the per-user pending cap.
func SubmitContribution(c *gin.Context) {
	setting := operation_setting.GetContributionSetting()
	if !setting.Enabled {
		common.ApiErrorMsg(c, "渠道贡献功能未开启")
		return
	}
	userId := c.GetInt("id")
	if userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户")
		return
	}
	var req submitContributionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	contribution, models, err := buildContributionFromRequest(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	contribution.UserId = userId
	if setting.MaxPendingPerUser > 0 {
		pending, err := model.CountPendingContributionsByUser(userId)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if pending >= int64(setting.MaxPendingPerUser) {
			common.ApiErrorMsg(c, "待审核的提交数量已达上限")
			return
		}
	}
	if err := model.InsertContributionWithModels(contribution, models); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": contribution.Id})
}

// GetSelfContributions handles GET /api/contribution/self — the caller's own
// submissions, keys masked.
func GetSelfContributions(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	contributions, total, err := model.ListContributionsByUser(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	for i := range contributions {
		contributions[i].Key = model.FormatContributionKey(contributions[i].Key)
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(contributions)
	common.ApiSuccess(c, pageInfo)
}

//__CONTRIB_CTRL_USER2__

// GetSelfContribution handles GET /api/contribution/detail/:id for the owner.
func GetSelfContribution(c *gin.Context) {
	userId := c.GetInt("id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	contribution, err := model.GetContributionById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if contribution.UserId != userId {
		common.ApiErrorMsg(c, "无权访问该提交")
		return
	}
	models, err := model.GetContributionModels(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	contribution.Key = model.FormatContributionKey(contribution.Key)
	common.ApiSuccess(c, contributionDetailView{ChannelContribution: *contribution, Models: models})
}

// UpdateSelfContribution handles PUT /api/contribution/detail/:id. Only pending
// submissions can be edited; both the connection details and the model list are
// rewritten. Ownership is enforced by UpdatePendingContributionDetails matching
// the user id, so ReplaceContributionModels only runs after that succeeds.
func UpdateSelfContribution(c *gin.Context) {
	userId := c.GetInt("id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req submitContributionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	contribution, models, err := buildContributionFromRequest(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdatePendingContributionDetails(id, userId, *contribution); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.ReplaceContributionModels(id, models); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// WithdrawSelfContribution handles DELETE /api/contribution/detail/:id.
func WithdrawSelfContribution(c *gin.Context) {
	userId := c.GetInt("id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.WithdrawContribution(id, userId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

//__CONTRIB_CTRL_EARN__

// transferContributionRequest moves earned share balance into spendable quota.
type transferContributionRequest struct {
	Amount int `json:"amount"`
}

// GetSelfContributionEarnings handles GET /api/contribution/earnings — the
// per-request share ledger plus the lifetime total and current share balance.
func GetSelfContributionEarnings(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	earnings, total, err := model.GetContributionEarnings(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	lifetime, err := model.SumContributionEarnings(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(earnings)
	common.ApiSuccess(c, gin.H{
		"page":                 pageInfo,
		"lifetime_share_quota": lifetime,
		"contribution_quota":   user.ContributionQuota,
	})
}

// GetSelfContributionShare handles GET /api/contribution/self/share, returning
// the caller's effective share percent: their per-user override when set,
// otherwise the global default.
func GetSelfContributionShare(c *gin.Context) {
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	percent := operation_setting.GetContributionSetting().DefaultSharePercent
	if user.ContributionSharePercent != nil {
		percent = *user.ContributionSharePercent
	}
	common.ApiSuccess(c, gin.H{"share_percent": percent})
}

// TransferContributionEarnings handles POST /api/contribution/transfer, moving
// share earnings into the caller's spendable wallet quota.
func TransferContributionEarnings(c *gin.Context) {
	userId := c.GetInt("id")
	var req transferContributionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Amount <= 0 {
		common.ApiErrorMsg(c, "转移额度必须大于 0")
		return
	}
	user := &model.User{}
	user.Id = userId
	if err := user.TransferContributionQuotaToQuota(req.Amount); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"contribution_quota": user.ContributionQuota,
		"quota":              user.Quota,
	})
}

//__CONTRIB_CTRL_ADMIN__

// GetContributionsForReview handles GET /api/contribution/admin?status= — the
// admin review queue, defaulting to pending submissions.
func GetContributionsForReview(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	status := model.ContributionStatusPending
	if s := c.Query("status"); s != "" {
		if parsed, err := strconv.Atoi(s); err == nil {
			status = parsed
		}
	}
	contributions, total, err := model.ListContributionsByStatus(status, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	for i := range contributions {
		contributions[i].Key = model.FormatContributionKey(contributions[i].Key)
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(contributions)
	common.ApiSuccess(c, pageInfo)
}

// GetContributionDetail handles GET /api/contribution/admin/:id.
func GetContributionDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	contribution, err := model.GetContributionById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	models, err := model.GetContributionModels(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	contribution.Key = model.FormatContributionKey(contribution.Key)
	common.ApiSuccess(c, contributionDetailView{ChannelContribution: *contribution, Models: models})
}

// reviewContributionRequest is the admin approve/reject payload. Decisions are
// optional per-model verdicts; an empty list approves every model.
type reviewContributionRequest struct {
	Id           int                                `json:"id"`
	Action       string                             `json:"action"`
	SharePercent *int                               `json:"share_percent"`
	RejectReason string                             `json:"reject_reason"`
	Decisions    []model.ContributionReviewDecision `json:"decisions"`
}

// ReviewContribution handles POST /api/contribution/admin/review. Approval
// materializes a live channel; rejection records the reviewer's reason.
func ReviewContribution(c *gin.Context) {
	reviewerId := c.GetInt("id")
	var req reviewContributionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Id <= 0 {
		common.ApiErrorMsg(c, "无效的提交 ID")
		return
	}
	if req.SharePercent != nil {
		normalized, err := operation_setting.NormalizeSharePercent(*req.SharePercent)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		req.SharePercent = &normalized
	}
	switch req.Action {
	case "approve":
		contribution, channelId, err := model.ApproveContribution(req.Id, reviewerId, req.SharePercent, req.Decisions)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, gin.H{"status": contribution.Status, "channel_id": channelId})
	case "reject":
		if err := model.RejectContribution(req.Id, reviewerId, req.RejectReason); err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, nil)
	default:
		common.ApiErrorMsg(c, "无效的操作")
	}
}

//__CONTRIB_CTRL_ADMIN2__

// GetContributors handles GET /api/contribution/admin/contributors.
func GetContributors(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	contributors, total, err := model.ListContributors(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(contributors)
	common.ApiSuccess(c, pageInfo)
}

// GetContributionStats handles GET /api/contribution/admin/stats — a status →
// count map for the admin overview.
func GetContributionStats(c *gin.Context) {
	counts, err := model.ContributionStatusCounts()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, counts)
}

// setSharePercentRequest sets a per-user share override; a nil percent clears
// it so the user inherits the global default.
type setSharePercentRequest struct {
	UserId       int  `json:"user_id"`
	SharePercent *int `json:"share_percent"`
}

// SetContributorSharePercent handles PUT /api/contribution/admin/share.
func SetContributorSharePercent(c *gin.Context) {
	var req setSharePercentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.UserId <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}
	if req.SharePercent != nil {
		normalized, err := operation_setting.NormalizeSharePercent(*req.SharePercent)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		req.SharePercent = &normalized
	}
	if err := model.UpdateUserContributionSharePercent(req.UserId, req.SharePercent); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
