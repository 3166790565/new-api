package controller

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
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

// stubOAuthProvider is a minimal oauth.Provider backed by the users.github_id
// column, used to drive the OAuth registration-code gate without a real IdP.
type stubOAuthProvider struct{ enabled bool }

func (stubOAuthProvider) GetName() string   { return "StubOAuth" }
func (p stubOAuthProvider) IsEnabled() bool { return p.enabled }
func (stubOAuthProvider) ExchangeToken(context.Context, string, *gin.Context) (*oauth.OAuthToken, error) {
	return nil, errors.New("not used in tests")
}
func (stubOAuthProvider) GetUserInfo(context.Context, *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	return nil, errors.New("not used in tests")
}
func (stubOAuthProvider) IsUserIDTaken(providerUserID string) bool {
	if providerUserID == "" {
		return false
	}
	var count int64
	model.DB.Model(&model.User{}).Where("github_id = ?", providerUserID).Count(&count)
	return count > 0
}
func (stubOAuthProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	return model.DB.Where("github_id = ?", providerUserID).First(user).Error
}
func (stubOAuthProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.GitHubId = providerUserID
}
func (stubOAuthProvider) GetProviderPrefix() string    { return "stub_" }
func (stubOAuthProvider) ProviderUserIDColumn() string { return "github_id" }

// registerStubOAuthProvider registers the stub under the given slug for the
// duration of the test.
func registerStubOAuthProvider(t *testing.T, slug string) {
	t.Helper()
	previous := oauth.GetProvider(slug)
	oauth.Register(slug, stubOAuthProvider{enabled: true})
	t.Cleanup(func() {
		if previous != nil {
			oauth.Register(slug, previous)
		} else {
			oauth.Unregister(slug)
		}
	})
}

// setupOAuthRegistrationTest builds a self-contained in-memory database with the
// tables the OAuth login and completion paths touch (including session issuance)
// and turns the registration-code gate on. Using :memory: with a single
// connection keeps the fixture free of the file-locking cleanup issues a
// file-backed sqlite log DB hits on Windows.
func setupOAuthRegistrationTest(t *testing.T) {
	t.Helper()
	require.NoError(t, i18n.Init())
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedis, previousSecret := common.RedisEnabled, common.SessionSecret
	previousRegister, previousRegCode := common.RegisterEnabled, common.RegistrationCodeEnabled
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.SessionSecret = "oauth-registration-test-secret"
	common.RegisterEnabled = true
	common.RegistrationCodeEnabled = true
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMain, previousLog)
		common.RedisEnabled, common.SessionSecret = previousRedis, previousSecret
		common.RegisterEnabled = previousRegister
		common.RegistrationCodeEnabled = previousRegCode
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.RegistrationCode{}, &model.AuthFlow{}, &model.UserSession{}, &model.TwoFA{}, &model.PasskeyCredential{}, &model.AuditLog{}, &model.Option{}))
}

// invokeOAuthLogin drives handleOAuthLogin as the callback would after the
// provider round trip, for a login-intent flow bound to the given slug.
func invokeOAuthLogin(provider oauth.Provider, slug string, oauthUser *oauth.OAuthUser) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/oauth/"+slug+"?state=x", nil)
	payload, _ := common.Marshal(oauthFlowPayload{})
	flow := &model.AuthFlow{Provider: slug, Intent: model.AuthFlowIntentLogin, Payload: string(payload)}
	handleOAuthLogin(c, provider, oauthUser, nil, flow)
	return response
}

// TestOAuthLoginRegistrationCodeGate covers the callback branch: an existing
// identity logs in without a code, a brand-new identity is deferred to the
// registration-code page (no account yet), and with the gate off it is created
// and logged in immediately — the pre-fix behavior.
func TestOAuthLoginRegistrationCodeGate(t *testing.T) {
	setupOAuthRegistrationTest(t)
	const slug = "stub"
	registerStubOAuthProvider(t, slug)
	provider := stubOAuthProvider{enabled: true}

	// Existing identity → straight login, never asked for a code.
	existing := &model.User{Username: "stub-existing", AffCode: "stubex", Status: common.UserStatusEnabled, AuthVersion: 1, GitHubId: "gh-existing"}
	require.NoError(t, model.DB.Create(existing).Error)
	require.NoError(t, model.PublishUserAuthCache(existing.Id))
	response := invokeOAuthLogin(provider, slug, &oauth.OAuthUser{ProviderUserID: "gh-existing"})
	var login securityEnrollmentResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &login))
	assert.True(t, login.Success, response.Body.String())
	assert.NotEqual(t, oauthRegistrationCodeRequiredCode, login.Code)

	// New identity + gate on → deferred, no account, a parked oauth_register flow.
	response = invokeOAuthLogin(provider, slug, &oauth.OAuthUser{ProviderUserID: "gh-new"})
	var deferred securityEnrollmentResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &deferred))
	assert.False(t, deferred.Success, response.Body.String())
	assert.Equal(t, oauthRegistrationCodeRequiredCode, deferred.Code)
	var registerData struct {
		RegisterToken string `json:"register_token"`
		Provider      string `json:"provider"`
	}
	require.NoError(t, common.Unmarshal(deferred.Data, &registerData))
	assert.NotEmpty(t, registerData.RegisterToken)
	assert.Equal(t, slug, registerData.Provider)
	assert.False(t, provider.IsUserIDTaken("gh-new"), "no account should be created yet")
	parked, err := model.GetAuthFlow(registerData.RegisterToken, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuthRegister})
	require.NoError(t, err)
	assert.Equal(t, slug, parked.Provider)

	// New identity + gate off → created and logged in without a code (old path).
	common.RegistrationCodeEnabled = false
	response = invokeOAuthLogin(provider, slug, &oauth.OAuthUser{ProviderUserID: "gh-nocode", Username: "gh-nocode"})
	var created securityEnrollmentResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &created))
	assert.True(t, created.Success, response.Body.String())
	assert.True(t, provider.IsUserIDTaken("gh-nocode"))
}

// TestCompleteOAuthRegistration covers the standalone registration-code step:
// a missing or invalid code is rejected without consuming the parked flow, a
// valid code creates the account, atomically consumes the code and the flow and
// issues a session, and a replayed or unknown token is rejected.
func TestCompleteOAuthRegistration(t *testing.T) {
	setupOAuthRegistrationTest(t)
	const slug = "stub"
	registerStubOAuthProvider(t, slug)

	now := common.GetTimestamp()
	valid := model.RegistrationCode{Code: "OAUTHVALID", Name: "v", Status: common.RegistrationCodeStatusEnabled, MaxUses: 1, CreatedTime: now, UpdatedTime: now}
	require.NoError(t, model.DB.Create(&valid).Error)

	parkFlow := func(providerUserID string) string {
		seed := oauthNewUserSeed{ProviderUserID: providerUserID, Username: providerUserID}
		payload, err := common.Marshal(seed)
		require.NoError(t, err)
		token, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
			Purpose:   model.AuthFlowPurposeOAuthRegister,
			Provider:  slug,
			Intent:    model.AuthFlowIntentLogin,
			Payload:   string(payload),
			ExpiresAt: time.Now().Add(10 * time.Minute),
		})
		require.NoError(t, err)
		return token
	}

	post := func(token, code string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		body := `{"register_token":"` + token + `","registration_code":"` + code + `"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/oauth/register", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		CompleteOAuthRegistration(c)
		return response
	}

	accountCount := func(providerUserID string) int64 {
		var count int64
		require.NoError(t, model.DB.Model(&model.User{}).Where("github_id = ?", providerUserID).Count(&count).Error)
		return count
	}

	token := parkFlow("gh-complete")
	match := model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuthRegister}

	// Missing code: rejected, flow still usable, no account.
	response := post(token, "")
	var body securityEnrollmentResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	assert.False(t, body.Success, response.Body.String())
	_, err := model.GetAuthFlow(token, match)
	require.NoError(t, err, "flow must not be consumed on a missing code")
	assert.Zero(t, accountCount("gh-complete"))

	// Invalid code: same — retryable without burning the flow.
	response = post(token, "WRONGCODE")
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	assert.False(t, body.Success, response.Body.String())
	_, err = model.GetAuthFlow(token, match)
	require.NoError(t, err, "flow must not be consumed on an invalid code")
	assert.Zero(t, accountCount("gh-complete"))

	// Valid code: account created, code + flow consumed, session issued.
	response = post(token, "OAUTHVALID")
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	assert.True(t, body.Success, response.Body.String())
	assert.EqualValues(t, 1, accountCount("gh-complete"))
	var reloaded model.RegistrationCode
	require.NoError(t, model.DB.First(&reloaded, "code = ?", "OAUTHVALID").Error)
	assert.Equal(t, 1, reloaded.UsedCount)
	_, err = model.GetAuthFlow(token, match)
	require.ErrorIs(t, err, model.ErrAuthFlowConsumed)

	// Replaying the consumed token creates no second account.
	response = post(token, "OAUTHVALID")
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	assert.False(t, body.Success, response.Body.String())
	assert.EqualValues(t, 1, accountCount("gh-complete"))

	// Unknown token is rejected.
	response = post("bogus-token", "OAUTHVALID")
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	assert.False(t, body.Success, response.Body.String())
}
