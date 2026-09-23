package model

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// RegistrationCode 是管理员生成、新用户注册（密码注册 / OAuth 首次建号）时必须持有的准入凭证。
// 注册码本身不携带额度或分组等任何权益，成功与否只决定能否创建账号。
type RegistrationCode struct {
	Id                int            `json:"id" gorm:"primaryKey"`
	Code              string         `json:"code" gorm:"type:char(32);uniqueIndex;not null"` // 紧凑令牌（规范化后存储）
	Name              string         `json:"name" gorm:"type:varchar(64);index"`             // 备注/名称
	Prefix            string         `json:"prefix" gorm:"type:varchar(16);default:''"`
	Suffix            string         `json:"suffix" gorm:"type:varchar(16);default:''"`
	Status            int            `json:"status" gorm:"default:1"`   // 1=enabled, 2=disabled
	MaxUses           int            `json:"max_uses" gorm:"default:1"` // 0 表示不限次数
	UsedCount         int            `json:"used_count" gorm:"default:0"`
	CreatedBy         int            `json:"created_by"`
	ExpiredTime       int64          `json:"expired_time" gorm:"bigint"` // 过期时间，0 表示不过期
	CreatedTime       int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime       int64          `json:"updated_time" gorm:"bigint"`
	LastUsedTime      int64          `json:"last_used_time" gorm:"bigint"`
	DeletedAt         gorm.DeletedAt `gorm:"index"`
	Count             int            `json:"count" gorm:"-:all"`          // 请求专用：批量数量
	ManualCodes       []string       `json:"manual_codes" gorm:"-"`       // 请求专用：手动录入的自定义码
	Charset           string         `json:"charset" gorm:"-"`            // 请求专用：digits|uppercase|mixed
	Length            int            `json:"length" gorm:"-"`             // 请求专用：码长
	ExcludeConfusable bool           `json:"exclude_confusable" gorm:"-"` // 请求专用：排除易混字符
}

var (
	ErrRegistrationCodeNotProvided = errors.New("registration code not provided")
	ErrRegistrationCodeInvalid     = errors.New("invalid registration code")
	ErrRegistrationCodeDisabled    = errors.New("registration code is disabled")
	ErrRegistrationCodeExhausted   = errors.New("registration code has been used up")
	ErrRegistrationCodeExpired     = errors.New("registration code has expired")
)

// registrationCodeKeyCol 是 code 列的方言安全引用。code 不是保留字，但为了
// 与 redemption 的 key 列保持一致的写法，这里也显式加引号。
const registrationCodeKeyCol = "`code`"

func registrationCodeKeyColName() string {
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		return `"code"`
	}
	return registrationCodeKeyCol
}

// registrationCodeFullValueMatch 在查询上追加“完整值（前缀+码+后缀）等于 normalized”的条件。
//
// 注册码分发给用户时展示、复制的是完整值 prefix+code+suffix（见前端
// formatRegistrationCodeValue / 导出对话框），因此注册校验必须针对完整值匹配，
// 而不能只比较基码列——否则任何带前缀/后缀的注册码都会被判为无效。
// 存储时 prefix/suffix/code 均已规范化（去空白、大写），传入的 normalized 亦然，
// 故三者直接拼接后按等值比较即可；前缀/后缀为空时完整值退化为基码，同样匹配。
// 拼接语法按方言区分：MySQL 用 CONCAT，SQLite 与 PostgreSQL 用 ||。
func registrationCodeFullValueMatch(query *gorm.DB, normalized string) *gorm.DB {
	codeCol := registrationCodeKeyColName()
	if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		return query.Where("CONCAT(prefix, "+codeCol+", suffix) = ?", normalized)
	}
	return query.Where("prefix || "+codeCol+" || suffix = ?", normalized)
}

// NormalizeRegistrationCodeValue 对用户输入的注册码做规范化：去首尾空白并统一大写。
// 生成与匹配都使用规范化后的值。
func NormalizeRegistrationCodeValue(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

// FullValue 返回注册码的完整展示值（前缀 + 码 + 后缀），用于管理端导出/复制。
// 匹配（ConsumeRegistrationCode）只使用 Code 基码，不依赖前缀/后缀。
func (rc *RegistrationCode) FullValue() string {
	return NormalizeRegistrationCodeValue(rc.Prefix) + rc.Code + NormalizeRegistrationCodeValue(rc.Suffix)
}

func GetAllRegistrationCodes(startIdx int, num int) (codes []*RegistrationCode, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	err = tx.Model(&RegistrationCode{}).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	err = tx.Order("id desc").Limit(num).Offset(startIdx).Find(&codes).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return codes, total, nil
}

func SearchRegistrationCodes(keyword string, status string, startIdx int, num int) (codes []*RegistrationCode, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&RegistrationCode{})
	if keyword != "" {
		normalized := NormalizeRegistrationCodeValue(keyword)
		query = query.Where("name LIKE ? OR code LIKE ?", keyword+"%", normalized+"%")
	}
	if status != "" {
		if statusInt, parseErr := strconv.Atoi(status); parseErr == nil {
			query = query.Where("status = ?", statusInt)
		}
	}

	err = query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&codes).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return codes, total, nil
}

func GetRegistrationCodeById(id int) (*RegistrationCode, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	code := RegistrationCode{Id: id}
	err := DB.First(&code, "id = ?", id).Error
	return &code, err
}

func GetRegistrationCodeByCode(code string) (*RegistrationCode, error) {
	normalized := NormalizeRegistrationCodeValue(code)
	if normalized == "" {
		return nil, errors.New("registration code not provided")
	}
	rc := RegistrationCode{}
	err := registrationCodeFullValueMatch(DB, normalized).First(&rc).Error
	return &rc, err
}

func CountRegistrationCodeByCode(code string) (int64, error) {
	normalized := NormalizeRegistrationCodeValue(code)
	if normalized == "" {
		return 0, nil
	}
	var count int64
	err := DB.Model(&RegistrationCode{}).Where(registrationCodeKeyColName()+" = ?", normalized).Count(&count).Error
	return count, err
}

func (rc *RegistrationCode) Insert() error {
	if rc.Code == "" {
		return errors.New("registration code value is empty")
	}
	if rc.Name == "" {
		return errors.New("registration code name is required")
	}
	rc.Code = NormalizeRegistrationCodeValue(rc.Code)
	return DB.Create(rc).Error
}

// Update 更新可编辑字段；码值创建后不可变。
func (rc *RegistrationCode) Update() error {
	return DB.Model(rc).Select("name", "status", "prefix", "suffix", "max_uses", "expired_time", "updated_time").Updates(rc).Error
}

func (rc *RegistrationCode) Delete() error {
	return DB.Delete(rc).Error
}

func DeleteRegistrationCodeById(id int) error {
	if id == 0 {
		return errors.New("id 为空！")
	}
	rc := RegistrationCode{Id: id}
	err := DB.Where(&rc).First(&rc).Error
	if err != nil {
		return err
	}
	return rc.Delete()
}

// DeleteInvalidRegistrationCodes 删除停用、已过期、或已用尽（used_count >= max_uses 且 max_uses != 0）的注册码。
func DeleteInvalidRegistrationCodes() (int64, error) {
	now := common.GetTimestamp()
	result := DB.Where(
		"status = ? OR (expired_time != 0 AND expired_time < ?) OR (max_uses != 0 AND used_count >= max_uses)",
		common.RegistrationCodeStatusDisabled,
		now,
	).Delete(&RegistrationCode{})
	return result.RowsAffected, result.Error
}

// BatchDeleteRegistrationCodes 批量软删除。
func BatchDeleteRegistrationCodes(ids []int) (int64, error) {
	if len(ids) == 0 || len(ids) > 1000 {
		return 0, errors.New("select between 1 and 1000 registration codes")
	}
	for _, id := range ids {
		if id <= 0 {
			return 0, errors.New("registration code IDs must be positive")
		}
	}
	result := DB.Where("id IN ?", ids).Delete(&RegistrationCode{})
	return result.RowsAffected, result.Error
}

// ============================================================================
// 码值生成
// ============================================================================

const (
	registrationCodeLengthMin = 4
	registrationCodeLengthMax = 32
	registrationCodeNameMax   = 64
)

// Charset 常量。
const (
	RegistrationCodeCharsetDigits    = "digits"
	RegistrationCodeCharsetUppercase = "uppercase"
	RegistrationCodeCharsetMixed     = "mixed"
)

func DigitsCharset() string       { return "0123456789" }
func UpperLettersCharset() string { return "ABCDEFGHIJKLMNOPQRSTUVWXYZ" }
func MixedCaseCharset() string {
	return DigitsCharset() + UpperLettersCharset() + "abcdefghijklmnopqrstuvwxyz"
}

// ConfusableExcludedCharset 从给定字符集中剔除易混淆字符 0O1lI。
func ConfusableExcludedCharset(charset string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune("0O1lI", r) {
			return -1
		}
		return r
	}, charset)
}

// GenerateRegistrationCodeToken 用 crypto/rand 从 charset 中均匀采样生成长度为 length 的令牌。
func GenerateRegistrationCodeToken(charset string, length int) (string, error) {
	if charset == "" {
		return "", errors.New("charset must not be empty")
	}
	if length < registrationCodeLengthMin || length > registrationCodeLengthMax {
		return "", fmt.Errorf("length must be between %d and %d", registrationCodeLengthMin, registrationCodeLengthMax)
	}
	b := make([]byte, length)
	maxI := big.NewInt(int64(len(charset)))
	for i := range b {
		n, err := rand.Int(rand.Reader, maxI)
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}

// GenerateUniqueRegistrationCodeToken 循环生成令牌并查重；最终防撞由数据库唯一索引保证。
func GenerateUniqueRegistrationCodeToken(charset string, length int, maxAttempts int) (string, error) {
	if maxAttempts <= 0 {
		maxAttempts = 10
	}
	for i := 0; i < maxAttempts; i++ {
		token, err := GenerateRegistrationCodeToken(charset, length)
		if err != nil {
			return "", err
		}
		count, err := CountRegistrationCodeByCode(token)
		if err != nil {
			return "", err
		}
		if count == 0 {
			return token, nil
		}
	}
	return "", errors.New("failed to generate a unique registration code")
}

// RegistrationCodeGenerationParams 是管理员生成注册码时的一组规则。
type RegistrationCodeGenerationParams struct {
	Charset           string
	Length            int
	Count             int
	Prefix            string
	Suffix            string
	MaxUses           int
	ExpiredTime       int64
	ExcludeConfusable bool
}

// ValidateRegistrationCodeGenerationParams 校验生成参数。
func ValidateRegistrationCodeGenerationParams(p RegistrationCodeGenerationParams) error {
	switch p.Charset {
	case RegistrationCodeCharsetDigits, RegistrationCodeCharsetUppercase, RegistrationCodeCharsetMixed:
	default:
		return errors.New("invalid charset")
	}
	if p.Length < registrationCodeLengthMin || p.Length > registrationCodeLengthMax {
		return fmt.Errorf("length must be between %d and %d", registrationCodeLengthMin, registrationCodeLengthMax)
	}
	if p.Count <= 0 || p.Count > 500 {
		return errors.New("count must be between 1 and 500")
	}
	if len(p.Prefix) > 16 || len(p.Suffix) > 16 {
		return errors.New("prefix and suffix must not exceed 16 characters")
	}
	if p.Prefix != "" && !isAlphanumeric(p.Prefix) {
		return errors.New("prefix must contain only letters and digits")
	}
	if p.Suffix != "" && !isAlphanumeric(p.Suffix) {
		return errors.New("suffix must contain only letters and digits")
	}
	if p.MaxUses < 0 {
		return errors.New("max uses must not be negative")
	}
	baseCharset := DigitsCharset()
	switch p.Charset {
	case RegistrationCodeCharsetDigits:
		baseCharset = DigitsCharset()
	case RegistrationCodeCharsetUppercase:
		baseCharset = UpperLettersCharset()
	case RegistrationCodeCharsetMixed:
		baseCharset = MixedCaseCharset()
	}
	if p.ExcludeConfusable {
		baseCharset = ConfusableExcludedCharset(baseCharset)
	}
	if len(baseCharset) < registrationCodeLengthMin {
		return errors.New("charset too small after excluding confusable characters")
	}
	return nil
}

func isAlphanumeric(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// ============================================================================
// 原子消费
// ============================================================================

// RegistrationCodeNameMax 返回注册码名称最大长度。
func RegistrationCodeNameMax() int { return registrationCodeNameMax }

// RegistrationCodeCharsetToAlphabet 将字符集枚举映射为实际字母表。
func RegistrationCodeCharsetToAlphabet(charset string) string {
	switch charset {
	case RegistrationCodeCharsetDigits:
		return DigitsCharset()
	case RegistrationCodeCharsetUppercase:
		return UpperLettersCharset()
	case RegistrationCodeCharsetMixed:
		return MixedCaseCharset()
	default:
		return ""
	}
}

// IsRegistrationCodePrefixSuffixValid 校验前缀/后缀只含字母数字。
func IsRegistrationCodePrefixSuffixValid(s string) bool {
	return isAlphanumeric(s)
}

// InsertRegistrationCodesManual 手动录入一批注册码：逐行规范化、去重、跳过已存在。
// 返回 (创建数量, 已存在跳过数量, 输入内重复数量, error)。
func InsertRegistrationCodesManual(name string, rawCodes []string, maxUses int, expiredTime int64, createdBy int, now int64) (int, int, int, error) {
	if maxUses < 0 {
		return 0, 0, 0, errors.New("max uses must not be negative")
	}
	seen := make(map[string]bool, len(rawCodes))
	var unique []string
	duplicates := 0
	for _, raw := range rawCodes {
		normalized := NormalizeRegistrationCodeValue(raw)
		if normalized == "" {
			continue
		}
		if seen[normalized] {
			duplicates++
			continue
		}
		seen[normalized] = true
		unique = append(unique, normalized)
	}
	skipped := 0
	created := 0
	for _, code := range unique {
		count, err := CountRegistrationCodeByCode(code)
		if err != nil {
			return created, skipped, duplicates, err
		}
		if count > 0 {
			skipped++
			continue
		}
		rc := &RegistrationCode{
			Code:        code,
			Name:        name,
			Status:      common.RegistrationCodeStatusEnabled,
			MaxUses:     maxUses,
			UsedCount:   0,
			CreatedBy:   createdBy,
			ExpiredTime: expiredTime,
			CreatedTime: now,
			UpdatedTime: now,
		}
		if err := rc.Insert(); err != nil {
			return created, skipped, duplicates, err
		}
		created++
	}
	return created, skipped, duplicates, nil
}

// CheckRegistrationCodeUsable 只读校验注册码当前是否可用（不消费）。
// 用于注册门禁的软失败路径；真正的原子扣减在 ConsumeRegistrationCode。
func CheckRegistrationCodeUsable(code string) error {
	normalized := NormalizeRegistrationCodeValue(code)
	if normalized == "" {
		return ErrRegistrationCodeNotProvided
	}
	rc := &RegistrationCode{}
	err := registrationCodeFullValueMatch(DB, normalized).First(rc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRegistrationCodeInvalid
		}
		return err
	}
	if rc.Status != common.RegistrationCodeStatusEnabled {
		return ErrRegistrationCodeDisabled
	}
	if rc.MaxUses != 0 && rc.UsedCount >= rc.MaxUses {
		return ErrRegistrationCodeExhausted
	}
	if rc.ExpiredTime != 0 && rc.ExpiredTime < common.GetTimestamp() {
		return ErrRegistrationCodeExpired
	}
	return nil
}

// ConsumeRegistrationCode 原子地消费一次注册码的使用次数。
// 返回码的 ID 与 Name（供审计），永不返回码值本身。
// 并发安全：事务 + lockForUpdate + 条件 CAS；同一码并发注册只有一个成功，其余得到 ErrRegistrationCodeExhausted。
func ConsumeRegistrationCode(code string) (rcID int, rcName string, err error) {
	normalized := NormalizeRegistrationCodeValue(code)
	if normalized == "" {
		return 0, "", ErrRegistrationCodeNotProvided
	}
	rc := &RegistrationCode{}
	err = DB.Transaction(func(tx *gorm.DB) error {
		err := registrationCodeFullValueMatch(lockForUpdate(tx), normalized).First(rc).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRegistrationCodeInvalid
			}
			return err
		}
		if rc.Status != common.RegistrationCodeStatusEnabled {
			return ErrRegistrationCodeDisabled
		}
		if rc.ExpiredTime != 0 && rc.ExpiredTime < common.GetTimestamp() {
			return ErrRegistrationCodeExpired
		}
		if rc.MaxUses == 0 {
			// 不限次数：无条件递增
			return tx.Model(&RegistrationCode{}).
				Where("id = ?", rc.Id).
				Updates(map[string]any{
					"used_count":     gorm.Expr("used_count + 1"),
					"last_used_time": common.GetTimestamp(),
				}).Error
		}
		// 有限次数：CAS，仅当还有剩余次数时递增
		result := tx.Model(&RegistrationCode{}).
			Where("id = ? AND used_count < max_uses", rc.Id).
			Updates(map[string]any{
				"used_count":     gorm.Expr("used_count + 1"),
				"last_used_time": common.GetTimestamp(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrRegistrationCodeExhausted
		}
		return nil
	})
	if err != nil {
		return 0, "", err
	}
	return rc.Id, rc.Name, nil
}
