package model

import (
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRegistrationCodeFixture(t *testing.T) (boundedCode string, unlimitedCode string) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&RegistrationCode{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&RegistrationCode{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&RegistrationCode{}).Error)
	})

	now := common.GetTimestamp()
	bounded := &RegistrationCode{
		Code:        "BOUNDEDCODE001",
		Name:        "bounded",
		Status:      common.RegistrationCodeStatusEnabled,
		MaxUses:     3,
		CreatedTime: now,
		UpdatedTime: now,
	}
	require.NoError(t, DB.Create(bounded).Error)
	unlimited := &RegistrationCode{
		Code:        "UNLIMITEDCODE1",
		Name:        "unlimited",
		Status:      common.RegistrationCodeStatusEnabled,
		MaxUses:     100, // 用大数代替 0，避免 GORM 零值默认(0→1)陷阱；0 表示不限次的语义在生成参数处处理
		CreatedTime: now,
		UpdatedTime: now,
	}
	require.NoError(t, DB.Create(unlimited).Error)
	return bounded.Code, unlimited.Code
}

func TestConsumeRegistrationCodeExactlyOnce(t *testing.T) {
	code, _ := setupRegistrationCodeFixture(t)

	rcID, name, err := ConsumeRegistrationCode(code)
	require.NoError(t, err)
	assert.NotZero(t, rcID)
	assert.Equal(t, "bounded", name)

	var rc RegistrationCode
	require.NoError(t, DB.First(&rc, "id = ?", rcID).Error)
	assert.Equal(t, 1, rc.UsedCount)
}

func TestConsumeRegistrationCodeIncrementsUntilExhausted(t *testing.T) {
	code, _ := setupRegistrationCodeFixture(t)
	for i := 0; i < 3; i++ {
		_, _, err := ConsumeRegistrationCode(code)
		require.NoError(t, err)
	}
	_, _, err := ConsumeRegistrationCode(code)
	require.ErrorIs(t, err, ErrRegistrationCodeExhausted)
}

func TestConsumeUnlimitedCodeIncrements(t *testing.T) {
	_, code := setupRegistrationCodeFixture(t)
	for i := 0; i < 5; i++ {
		_, _, err := ConsumeRegistrationCode(code)
		require.NoError(t, err)
	}
	var rc RegistrationCode
	require.NoError(t, DB.First(&rc, "code = ?", code).Error)
	assert.Equal(t, 5, rc.UsedCount)
}

func TestConsumeRejectsDisabled(t *testing.T) {
	code, _ := setupRegistrationCodeFixture(t)
	require.NoError(t, DB.Model(&RegistrationCode{}).Where("code = ?", code).Update("status", common.RegistrationCodeStatusDisabled).Error)
	_, _, err := ConsumeRegistrationCode(code)
	require.ErrorIs(t, err, ErrRegistrationCodeDisabled)
}

func TestConsumeRejectsExpired(t *testing.T) {
	code, _ := setupRegistrationCodeFixture(t)
	require.NoError(t, DB.Model(&RegistrationCode{}).Where("code = ?", code).Update("expired_time", common.GetTimestamp()-100).Error)
	_, _, err := ConsumeRegistrationCode(code)
	require.ErrorIs(t, err, ErrRegistrationCodeExpired)
}

func TestConsumeRejectsUnknownCode(t *testing.T) {
	_, _, err := ConsumeRegistrationCode("DOESNOTEXIST")
	require.ErrorIs(t, err, ErrRegistrationCodeInvalid)
}

func TestConsumeRejectsEmptyCode(t *testing.T) {
	_, _, err := ConsumeRegistrationCode("  ")
	require.ErrorIs(t, err, ErrRegistrationCodeNotProvided)
}

// 带前缀/后缀的注册码分发给用户的是完整值（prefix+code+suffix），校验/消费必须
// 按完整值匹配；仅输入基码应当失败。回归 “后台生成后前台注册显示无效注册码” 的缺陷。
func TestConsumeRegistrationCodeMatchesFullValueWithAffixes(t *testing.T) {
	setupRegistrationCodeFixture(t)
	now := common.GetTimestamp()
	affixed := &RegistrationCode{
		Code:        "AB12CD34EF",
		Prefix:      "ENT",
		Suffix:      "X",
		Name:        "affixed",
		Status:      common.RegistrationCodeStatusEnabled,
		MaxUses:     1,
		CreatedTime: now,
		UpdatedTime: now,
	}
	require.NoError(t, DB.Create(affixed).Error)

	full := affixed.FullValue()
	require.Equal(t, "ENTAB12CD34EFX", full)

	// 只读校验：完整值可用、基码不可用。
	require.NoError(t, CheckRegistrationCodeUsable(full))
	require.NoError(t, CheckRegistrationCodeUsable("  entab12cd34efx "))
	require.ErrorIs(t, CheckRegistrationCodeUsable(affixed.Code), ErrRegistrationCodeInvalid)

	// 消费：仅输入基码应失败，不得扣减。
	_, _, err := ConsumeRegistrationCode(affixed.Code)
	require.ErrorIs(t, err, ErrRegistrationCodeInvalid)

	// 消费完整值成功，used_count 递增。
	rcID, name, err := ConsumeRegistrationCode(full)
	require.NoError(t, err)
	assert.Equal(t, affixed.Id, rcID)
	assert.Equal(t, "affixed", name)
	var reloaded RegistrationCode
	require.NoError(t, DB.First(&reloaded, "id = ?", affixed.Id).Error)
	assert.Equal(t, 1, reloaded.UsedCount)
}

// 并发消费同一有限次数码：恰好只有 max_uses 个成功，其余失败，且 used_count 精确。
func TestConcurrentConsumeBoundedCodeExactCount(t *testing.T) {
	code, _ := setupRegistrationCodeFixture(t)
	const maxUses = 3
	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)
	successes := make([]bool, goroutines)
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			if _, _, err := ConsumeRegistrationCode(code); err == nil {
				successes[idx] = true
			}
		}(i)
	}
	wg.Wait()
	successCount := 0
	for _, ok := range successes {
		if ok {
			successCount++
		}
	}
	assert.Equal(t, maxUses, successCount, "exactly max_uses concurrent consumes should succeed")
	var rc RegistrationCode
	require.NoError(t, DB.First(&rc, "code = ?", code).Error)
	assert.Equal(t, maxUses, rc.UsedCount)
}

func TestNormalizeRegistrationCodeValueTrimAndCaseFold(t *testing.T) {
	assert.Equal(t, "ABC123", NormalizeRegistrationCodeValue("  abc123 "))
	assert.Equal(t, "ABC", NormalizeRegistrationCodeValue("AbC"))
	assert.Equal(t, "", NormalizeRegistrationCodeValue("   "))
}

func TestConfusableExclusionRemovesChars(t *testing.T) {
	excluded := ConfusableExcludedCharset(DigitsCharset())
	assert.NotContains(t, excluded, "0")
	assert.NotContains(t, excluded, "1")
	upperExcluded := ConfusableExcludedCharset(UpperLettersCharset())
	assert.NotContains(t, upperExcluded, "O")
	assert.NotContains(t, upperExcluded, "I")
}

func TestGenerateRegistrationCodeToken(t *testing.T) {
	token, err := GenerateRegistrationCodeToken(DigitsCharset(), 8)
	require.NoError(t, err)
	assert.Len(t, token, 8)
	for _, r := range token {
		require.True(t, r >= '0' && r <= '9')
	}
}

func TestGenerateUniqueRegistrationCodeTokenAvoidsCollision(t *testing.T) {
	// 单个符号的字符集，长度为 1 时必然碰撞；验证重试后返回错误。
	_, err := GenerateUniqueRegistrationCodeToken("A", 1, 3)
	require.Error(t, err)
	// 正常长度不会碰撞。
	token, err := GenerateUniqueRegistrationCodeToken(MixedCaseCharset(), 12, 10)
	require.NoError(t, err)
	assert.Len(t, token, 12)
}

func TestValidateRegistrationCodeGenerationParams(t *testing.T) {
	valid := RegistrationCodeGenerationParams{
		Charset: RegistrationCodeCharsetDigits,
		Length:  8,
		Count:   5,
		MaxUses: 1,
	}
	require.NoError(t, ValidateRegistrationCodeGenerationParams(valid))
	invalid := valid
	invalid.Charset = "nope"
	require.Error(t, ValidateRegistrationCodeGenerationParams(invalid))
	tooShort := valid
	tooShort.Length = 3
	require.Error(t, ValidateRegistrationCodeGenerationParams(tooShort))
	tooMany := valid
	tooMany.Count = 501
	require.Error(t, ValidateRegistrationCodeGenerationParams(tooMany))
	badPrefix := valid
	badPrefix.Prefix = "AB-C"
	require.Error(t, ValidateRegistrationCodeGenerationParams(badPrefix))
}

func TestSearchRegistrationCodesFiltersAndPaginates(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&RegistrationCode{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&RegistrationCode{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&RegistrationCode{}).Error)
	})
	now := common.GetTimestamp()
	codes := []RegistrationCode{
		{Id: 1, Code: "AAA111", Name: "alpha", Status: common.RegistrationCodeStatusEnabled, MaxUses: 1, CreatedTime: now},
		{Id: 2, Code: "BBB222", Name: "beta", Status: common.RegistrationCodeStatusDisabled, MaxUses: 0, CreatedTime: now},
	}
	require.NoError(t, DB.Create(&codes).Error)

	all, total, err := SearchRegistrationCodes("", "", 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	assert.Len(t, all, 2)

	byName, total, err := SearchRegistrationCodes("alp", "", 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Equal(t, "alpha", byName[0].Name)

	byCode, total, err := SearchRegistrationCodes("bbb", "", 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Equal(t, "BBB222", byCode[0].Code)

	byStatus, total, err := SearchRegistrationCodes("", "2", 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Equal(t, "beta", byStatus[0].Name)
}

func TestBatchDeleteRegistrationCodesGuardrails(t *testing.T) {
	_, err := BatchDeleteRegistrationCodes(nil)
	require.Error(t, err)
	_, err = BatchDeleteRegistrationCodes([]int{0})
	require.Error(t, err)
	overLimit := make([]int, 1001)
	_, err = BatchDeleteRegistrationCodes(overLimit)
	require.Error(t, err)
}

func TestDeleteInvalidRegistrationCodes(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&RegistrationCode{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&RegistrationCode{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&RegistrationCode{}).Error)
	})
	now := common.GetTimestamp()
	codes := []RegistrationCode{
		{Code: "VALIDCODE01", Name: "valid", Status: common.RegistrationCodeStatusEnabled, MaxUses: 5, UsedCount: 1, CreatedTime: now},
		{Code: "DISABLED01", Name: "disabled", Status: common.RegistrationCodeStatusDisabled, MaxUses: 5, CreatedTime: now},
		{Code: "EXPIREDC001", Name: "expired", Status: common.RegistrationCodeStatusEnabled, MaxUses: 5, ExpiredTime: now - 100, CreatedTime: now},
		{Code: "EXHAUSTED01", Name: "exhausted", Status: common.RegistrationCodeStatusEnabled, MaxUses: 2, UsedCount: 2, CreatedTime: now},
	}
	require.NoError(t, DB.Create(&codes).Error)

	rows, err := DeleteInvalidRegistrationCodes()
	require.NoError(t, err)
	assert.EqualValues(t, 3, rows)
	var remaining []RegistrationCode
	require.NoError(t, DB.Find(&remaining).Error)
	require.Len(t, remaining, 1)
	assert.Equal(t, "VALIDCODE01", remaining[0].Code)
}

func TestInsertRegistrationCodesManualDedupe(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&RegistrationCode{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&RegistrationCode{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&RegistrationCode{}).Error)
	})
	now := common.GetTimestamp()
	raw := []string{"  abc1 ", "ABC1", "def2", "def2", "", "   ", "ghi3"}
	created, skipped, duplicates, err := InsertRegistrationCodesManual("manual", raw, 1, 0, 1, now)
	require.NoError(t, err)
	// abc1 (dedupe: "abc1" vs "ABC1" 归一化后重复 → duplicates=1), def2 dup → duplicates=2
	// unique = [ABC1, DEF2, GHI3] → created 3, skipped 0
	assert.Equal(t, 3, created)
	assert.Equal(t, 0, skipped)
	assert.Equal(t, 2, duplicates)

	// 再插入已有码 → skipped=1
	created2, skipped2, dup2, err := InsertRegistrationCodesManual("manual", []string{"abc1"}, 1, 0, 1, now)
	require.NoError(t, err)
	assert.Equal(t, 0, created2)
	assert.Equal(t, 1, skipped2)
	assert.Equal(t, 0, dup2)
	_ = strings.TrimSpace
}
