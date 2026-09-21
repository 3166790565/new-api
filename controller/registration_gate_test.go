package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestRegisterRequiresRegistrationCode 覆盖密码注册的注册码门禁：
// 缺码/无效/停用/过期/用尽都拒绝，且不创建账号；有效码则创建并原子消费。
func TestRegisterRequiresRegistrationCode(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRegister, previousRegCode := common.RegisterEnabled, common.RegistrationCodeEnabled
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.RegisterEnabled = true
	common.RegistrationCodeEnabled = true
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RegisterEnabled = previousRegister
		common.RegistrationCodeEnabled = previousRegCode
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.RegistrationCode{}))

	now := common.GetTimestamp()
	valid := model.RegistrationCode{Code: "VALIDCODE1", Name: "v", Status: common.RegistrationCodeStatusEnabled, MaxUses: 1, CreatedTime: now, UpdatedTime: now}
	require.NoError(t, db.Create(&valid).Error)
	disabled := model.RegistrationCode{Code: "DISABLED01", Name: "d", Status: common.RegistrationCodeStatusDisabled, MaxUses: 1, CreatedTime: now, UpdatedTime: now}
	require.NoError(t, db.Create(&disabled).Error)
	expired := model.RegistrationCode{Code: "EXPIREDC01", Name: "e", Status: common.RegistrationCodeStatusEnabled, MaxUses: 1, ExpiredTime: now - 100, CreatedTime: now, UpdatedTime: now}
	require.NoError(t, db.Create(&expired).Error)
	usedUp := model.RegistrationCode{Code: "USEDUP001", Name: "u", Status: common.RegistrationCodeStatusEnabled, MaxUses: 1, UsedCount: 1, CreatedTime: now, UpdatedTime: now}
	require.NoError(t, db.Create(&usedUp).Error)

	router := gin.New()
	router.POST("/api/user/register", Register)

	registerBody := func(code string) string {
		return `{"username":"reg-user","password":"password123","registration_code":"` + code + `"}`
	}

	cases := []struct {
		name          string
		code          string
		wantSuccess   bool
		wantNoAccount bool
	}{
		{"missing code", "", false, true},
		{"invalid code", "DOESNOTEXIST", false, true},
		{"disabled code", "DISABLED01", false, true},
		{"expired code", "EXPIREDC01", false, true},
		{"used up code", "USEDUP001", false, true},
		{"valid code", "VALIDCODE1", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBufferString(registerBody(tc.code)))
			router.ServeHTTP(response, request)
			var result struct {
				Success bool `json:"success"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
			assert.Equal(t, tc.wantSuccess, result.Success, response.Body.String())
			var userCount int64
			require.NoError(t, db.Model(&model.User{}).Where("username = ?", "reg-user").Count(&userCount).Error)
			if tc.wantNoAccount {
				assert.Zero(t, userCount)
			} else {
				assert.EqualValues(t, 1, userCount)
			}
		})
	}

	// 有效码被消费后 used_count=1，再次使用被拒。
	var reloaded model.RegistrationCode
	require.NoError(t, db.First(&reloaded, "code = ?", "VALIDCODE1").Error)
	assert.Equal(t, 1, reloaded.UsedCount)
}

// TestRegisterFeatureDisabledBehavesAsBefore 开关关闭时，无注册码也能注册（旧行为）。
func TestRegisterFeatureDisabledBehavesAsBefore(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRegister, previousRegCode := common.RegisterEnabled, common.RegistrationCodeEnabled
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.RegisterEnabled = true
	common.RegistrationCodeEnabled = false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RegisterEnabled = previousRegister
		common.RegistrationCodeEnabled = previousRegCode
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.RegistrationCode{}))

	router := gin.New()
	router.POST("/api/user/register", Register)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBufferString(`{"username":"legacy-user","password":"password123"}`))
	router.ServeHTTP(response, request)
	var result struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	assert.True(t, result.Success, response.Body.String())
}
