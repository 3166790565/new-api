package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestDeleteRegistrationCodeBatch(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver, logDriver gorm.Dialector
			dbType := common.DatabaseTypeSQLite
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(":memory:")
				logDriver = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				driver = mysql.Open(dsn)
				logDriver = mysql.Open(dsn)
				dbType = common.DatabaseTypeMySQL
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				driver = postgres.Open(dsn)
				logDriver = postgres.Open(dsn)
				dbType = common.DatabaseTypePostgreSQL
			}
			db, err := gorm.Open(driver, &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

			logDB, err := gorm.Open(logDriver, &gorm.Config{})
			require.NoError(t, err)
			logSQL, err := logDB.DB()
			require.NoError(t, err)
			logSQL.SetMaxOpenConns(1)
			t.Cleanup(func() { require.NoError(t, logSQL.Close()) })

			previousDB, previousLogDB := model.DB, model.LOG_DB
			previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
			previousRedis := common.RedisEnabled
			model.DB, model.LOG_DB = db, logDB
			common.SetDatabaseTypes(dbType, dbType)
			common.RedisEnabled = false
			t.Cleanup(func() {
				model.DB, model.LOG_DB = previousDB, previousLogDB
				common.SetDatabaseTypes(previousMain, previousLog)
				common.RedisEnabled = previousRedis
			})
			for _, table := range []any{&model.User{}, &model.RegistrationCode{}} {
				require.False(t, db.Migrator().HasTable(table), "use an empty test database")
				require.NoError(t, db.AutoMigrate(table))
				t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable(table)) })
			}
			require.False(t, logDB.Migrator().HasTable(&model.AuditLog{}), "use an empty test log database")
			require.NoError(t, logDB.AutoMigrate(&model.AuditLog{}))
			t.Cleanup(func() { require.NoError(t, logDB.Migrator().DropTable(&model.AuditLog{})) })

			token := "registration-code-audit-test-token"
			admin := model.User{Username: "registration-code-audit-admin", Password: "unused", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Group: "default", AccessToken: &token}
			require.NoError(t, db.Create(&admin).Error)

			now := common.GetTimestamp()
			codes := make([]model.RegistrationCode, 16)
			for index := range codes {
				codes[index] = model.RegistrationCode{Code: fmt.Sprintf("CODE%08d", index+1), Name: "selected", Status: common.RegistrationCodeStatusEnabled, MaxUses: 1, CreatedTime: now, UpdatedTime: now, CreatedBy: admin.Id}
			}
			codes[15].Name = "unselected"
			require.NoError(t, model.DB.Create(&codes).Error)

			router := gin.New()
			router.Use(middleware.RequestId())
			router.POST("/api/registration_code/batch", middleware.AdminAuth(), DeleteRegistrationCodeBatch)

			overLimit := make([]int, 1001)
			for index := range overLimit {
				overLimit[index] = codes[0].Id
			}
			oversized, err := common.Marshal(map[string]any{"ids": overLimit})
			require.NoError(t, err)
			for _, body := range []string{"{}", `{"ids":[]}`, `{"ids":null}`, `{"ids":[0]}`, `{"ids":[1,-1]}`, `{"ids":["1"]}`, "{", string(oversized)} {
				t.Run("invalid_"+body[:min(len(body), 30)], func(t *testing.T) {
					response := httptest.NewRecorder()
					request := httptest.NewRequest(http.MethodPost, "/api/registration_code/batch", bytes.NewBufferString(body))
					request.Header.Set("Authorization", "Bearer "+token)
					router.ServeHTTP(response, request)
					var result struct {
						Success bool `json:"success"`
					}
					require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
					assert.False(t, result.Success)
					var count int64
					require.NoError(t, model.DB.Model(&model.RegistrationCode{}).Count(&count).Error)
					assert.EqualValues(t, 16, count)
				})
			}
			requestedIDs := make([]int, 0, 15)
			for _, code := range codes[:15] {
				requestedIDs = append(requestedIDs, code.Id)
			}
			payload, err := common.Marshal(map[string]any{"ids": requestedIDs})
			require.NoError(t, err)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/registration_code/batch", bytes.NewReader(payload))
			request.Header.Set("Authorization", "Bearer "+token)
			router.ServeHTTP(response, request)
			assert.Equal(t, http.StatusOK, response.Code)
			var result struct {
				Success bool  `json:"success"`
				Data    int64 `json:"data"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
			assert.True(t, result.Success)
			assert.Equal(t, int64(15), result.Data)
			var active []model.RegistrationCode
			require.NoError(t, model.DB.Find(&active).Error)
			require.Len(t, active, 1)
			assert.Equal(t, "unselected", active[0].Name)
		})
	}
}

// 匿名请求必须被拒绝（AdminAuth 保护）。
func TestRegistrationCodeRoutesRequireAdmin(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	previousDB, previousLogDB := model.DB, model.LOG_DB
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
	})

	router := gin.New()
	router.Use(middleware.AdminAuth())
	router.GET("/api/registration_code/", GetAllRegistrationCodes)
	router.POST("/api/registration_code/", AddRegistrationCode)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/registration_code/", nil)
	router.ServeHTTP(response, request)
	assert.Equal(t, http.StatusUnauthorized, response.Code)

	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/registration_code/", bytes.NewBufferString(`{}`))
	router.ServeHTTP(response, request)
	assert.Equal(t, http.StatusUnauthorized, response.Code)
}
