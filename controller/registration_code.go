package controller

import (
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetAllRegistrationCodes(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	codes, total, err := model.GetAllRegistrationCodes(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(codes)
	common.ApiSuccess(c, pageInfo)
}

func SearchRegistrationCodes(c *gin.Context) {
	keyword := c.Query("keyword")
	status := c.Query("status")
	pageInfo := common.GetPageQuery(c)
	codes, total, err := model.SearchRegistrationCodes(keyword, status, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(codes)
	common.ApiSuccess(c, pageInfo)
}

func GetRegistrationCode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	code, err := model.GetRegistrationCodeById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, code)
}

func AddRegistrationCode(c *gin.Context) {
	rc := model.RegistrationCode{}
	if err := c.ShouldBindJSON(&rc); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if utf8.RuneCountInString(rc.Name) == 0 || utf8.RuneCountInString(rc.Name) > model.RegistrationCodeNameMax() {
		common.ApiErrorI18n(c, i18n.MsgRegistrationCodeNameLength)
		return
	}
	now := common.GetTimestamp()

	// 手动录入分支：请求体携带 ManualCodes 时，直接按行录入（自动去重、跳过已存在）。
	if len(rc.ManualCodes) > 0 {
		created, skipped, duplicates, err := model.InsertRegistrationCodesManual(rc.Name, rc.ManualCodes, rc.MaxUses, rc.ExpiredTime, c.GetInt("id"), now)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		recordManageAudit(c, "registration_code.create_manual", map[string]any{
			"name":       rc.Name,
			"count":      created,
			"skipped":    skipped,
			"duplicates": duplicates,
		})
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data": map[string]any{
				"created":    created,
				"skipped":    skipped,
				"duplicates": duplicates,
			},
		})
		return
	}

	// 随机批量生成分支
	if rc.MaxUses < 0 {
		common.ApiErrorI18n(c, i18n.MsgRegistrationCodeMaxUsesInvalid)
		return
	}
	params := model.RegistrationCodeGenerationParams{
		Charset:           rc.Charset,
		Length:            rc.Length,
		Count:             rc.Count,
		Prefix:            rc.Prefix,
		Suffix:            rc.Suffix,
		MaxUses:           rc.MaxUses,
		ExpiredTime:       rc.ExpiredTime,
		ExcludeConfusable: rc.ExcludeConfusable,
	}
	if err := model.ValidateRegistrationCodeGenerationParams(params); err != nil {
		common.ApiErrorI18n(c, i18n.MsgRegistrationCodeInvalidParams)
		return
	}
	charset := model.RegistrationCodeCharsetToAlphabet(params.Charset)
	if params.ExcludeConfusable {
		charset = model.ConfusableExcludedCharset(charset)
	}

	var keys []string
	for i := 0; i < params.Count; i++ {
		code, err := model.GenerateUniqueRegistrationCodeToken(charset, params.Length, 10)
		if err != nil {
			common.SysError("failed to generate unique registration code: " + err.Error())
			common.ApiErrorI18n(c, i18n.MsgRegistrationCodeCreateFailed)
			return
		}
		clean := model.RegistrationCode{
			Code:        code,
			Name:        rc.Name,
			Prefix:      model.NormalizeRegistrationCodeValue(params.Prefix),
			Suffix:      model.NormalizeRegistrationCodeValue(params.Suffix),
			Status:      common.RegistrationCodeStatusEnabled,
			MaxUses:     params.MaxUses,
			UsedCount:   0,
			CreatedBy:   c.GetInt("id"),
			ExpiredTime: params.ExpiredTime,
			CreatedTime: now,
			UpdatedTime: now,
		}
		if err := clean.Insert(); err != nil {
			common.SysError("failed to insert registration code: " + err.Error())
			common.ApiErrorI18n(c, i18n.MsgRegistrationCodeCreateFailed)
			return
		}
		keys = append(keys, code)
	}

	recordManageAudit(c, "registration_code.create", map[string]any{
		"name":               rc.Name,
		"count":              params.Count,
		"length":             params.Length,
		"charset":            params.Charset,
		"exclude_confusable": params.ExcludeConfusable,
		"prefix":             params.Prefix,
		"suffix":             params.Suffix,
		"max_uses":           params.MaxUses,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    keys,
	})
}

func UpdateRegistrationCode(c *gin.Context) {
	statusOnly := c.Query("status_only")
	rc := model.RegistrationCode{}
	if err := c.ShouldBindJSON(&rc); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	clean, err := model.GetRegistrationCodeById(rc.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if statusOnly == "" {
		if utf8.RuneCountInString(rc.Name) == 0 || utf8.RuneCountInString(rc.Name) > model.RegistrationCodeNameMax() {
			common.ApiErrorI18n(c, i18n.MsgRegistrationCodeNameLength)
			return
		}
		if rc.MaxUses < 0 {
			common.ApiErrorI18n(c, i18n.MsgRegistrationCodeMaxUsesInvalid)
			return
		}
		// 若开启了生成规则修改，重新校验（前缀/后缀/用次/过期）
		params := model.RegistrationCodeGenerationParams{
			Charset:           rc.Charset,
			Length:            rc.Length,
			Count:             rc.Count,
			Prefix:            rc.Prefix,
			Suffix:            rc.Suffix,
			MaxUses:           rc.MaxUses,
			ExpiredTime:       rc.ExpiredTime,
			ExcludeConfusable: rc.ExcludeConfusable,
		}
		_ = params // charset/length/count 在更新时不可变，忽略
		if rc.Prefix != "" && !model.IsRegistrationCodePrefixSuffixValid(rc.Prefix) {
			common.ApiErrorI18n(c, i18n.MsgRegistrationCodeInvalidParams)
			return
		}
		if rc.Suffix != "" && !model.IsRegistrationCodePrefixSuffixValid(rc.Suffix) {
			common.ApiErrorI18n(c, i18n.MsgRegistrationCodeInvalidParams)
			return
		}
		if rc.ExpiredTime != 0 && rc.ExpiredTime < common.GetTimestamp() {
			common.ApiErrorI18n(c, i18n.MsgRegistrationCodeExpireTimeInvalid)
			return
		}
		clean.Name = rc.Name
		clean.Prefix = model.NormalizeRegistrationCodeValue(rc.Prefix)
		clean.Suffix = model.NormalizeRegistrationCodeValue(rc.Suffix)
		clean.MaxUses = rc.MaxUses
		clean.ExpiredTime = rc.ExpiredTime
	}
	if statusOnly != "" {
		if rc.Status != common.RegistrationCodeStatusEnabled && rc.Status != common.RegistrationCodeStatusDisabled {
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return
		}
		clean.Status = rc.Status
	}
	clean.UpdatedTime = common.GetTimestamp()
	if err := clean.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	action := "registration_code.update"
	params := map[string]any{"id": clean.Id}
	if statusOnly != "" {
		if clean.Status == common.RegistrationCodeStatusEnabled {
			action = "registration_code.enable"
			params["enabled"] = true
		} else {
			action = "registration_code.disable"
			params["enabled"] = false
		}
	}
	recordManageAudit(c, action, params)
	common.ApiSuccess(c, clean)
}

func DeleteRegistrationCode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err := model.DeleteRegistrationCodeById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "registration_code.delete", map[string]any{"id": id})
	common.ApiSuccess(c, nil)
}

func DeleteInvalidRegistrationCode(c *gin.Context) {
	rows, err := model.DeleteInvalidRegistrationCodes()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "registration_code.delete_invalid", map[string]any{"count": rows})
	common.ApiSuccess(c, rows)
}

func DeleteRegistrationCodeBatch(c *gin.Context) {
	var request struct {
		Ids []int `json:"ids" binding:"required,min=1,max=1000,dive,gt=0"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	count, err := model.BatchDeleteRegistrationCodes(request.Ids)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "registration_code.delete_batch", map[string]any{
		"count":                           count,
		"total":                           len(request.Ids),
		"requested_registration_code_ids": request.Ids,
	})
	common.ApiSuccess(c, count)
}
