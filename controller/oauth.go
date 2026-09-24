package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const oauthAuthFlowTTL = 10 * time.Minute

type oauthStateRequest struct {
	Provider         string          `json:"provider"`
	Intent           string          `json:"intent"`
	Aff              string          `json:"aff,omitempty"`
	Scope            string          `json:"scope,omitempty"`
	Context          json.RawMessage `json:"context,omitempty"`
	RegistrationCode string          `json:"registration_code,omitempty"`
}

type oauthFlowPayload struct {
	AffiliateCode   string                         `json:"affiliate_code,omitempty"`
	Verification    *service.OAuthVerificationFlow `json:"verification,omitempty"`
	Telegram        *oauth.TelegramOAuthFlow       `json:"telegram,omitempty"`
	SessionIdentity *service.AuthIdentity          `json:"session_identity,omitempty"`
	Authorization   *model.AuthFlowAuthorization   `json:"authorization,omitempty"`
}

// oauthNewUserSeed carries a provider identity the callback has already
// verified but not yet turned into an account, so registration-code entry can
// complete the sign-up on a separate step. It holds no secrets and no code.
type oauthNewUserSeed struct {
	ProviderUserID string `json:"provider_user_id"`
	Username       string `json:"username,omitempty"`
	DisplayName    string `json:"display_name,omitempty"`
	Email          string `json:"email,omitempty"`
	AffiliateCode  string `json:"affiliate_code,omitempty"`
}

// oauthRegistrationCodeRequiredCode is the machine code the OAuth callback
// returns when a new account needs a registration code; the frontend routes on
// it to the standalone registration-code page.
const oauthRegistrationCodeRequiredCode = "REGISTRATION_CODE_REQUIRED"

// providerParams returns map with Provider key for i18n templates
func providerParams(name string) map[string]any {
	return map[string]any{"Provider": name}
}

// GenerateOAuthCode generates a state code for OAuth CSRF protection
func GenerateOAuthCode(c *gin.Context) {
	var request oauthStateRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	request.Provider = strings.TrimSpace(request.Provider)
	request.Intent = strings.TrimSpace(request.Intent)
	request.Aff = strings.TrimSpace(request.Aff)
	if oauth.GetProvider(request.Provider) == nil ||
		(request.Intent != model.AuthFlowIntentLogin && request.Intent != model.AuthFlowIntentBind && request.Intent != model.AuthFlowIntentVerify) ||
		len(request.Aff) > 32 ||
		(request.Intent != model.AuthFlowIntentLogin && request.Aff != "") ||
		(request.Intent != model.AuthFlowIntentVerify && (request.Scope != "" || len(request.Context) != 0)) {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	userID := 0
	sessionID := ""
	flowPayload := oauthFlowPayload{AffiliateCode: request.Aff}
	bindingStarted := false
	if request.Provider == "telegram" {
		telegramFlow, err := oauth.NewTelegramOAuthFlow()
		if err != nil {
			writeSecurityOperationError(c, err)
			return
		}
		flowPayload.Telegram = telegramFlow
	}
	if request.Intent == model.AuthFlowIntentBind || request.Intent == model.AuthFlowIntentVerify {
		identity, ok := middleware.GetSessionAuthIdentity(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "绑定操作需要登录"})
			return
		}
		userID = identity.UserID
		sessionID = identity.SessionID
		if request.Intent == model.AuthFlowIntentBind {
			defer func() {
				recordUserSecurityAudit(c, userID, "user.binding_start", map[string]any{"provider": request.Provider, "success": bindingStarted})
			}()
			context, err := common.Marshal(service.AccountBindingContext{Provider: request.Provider})
			if err != nil {
				writeSecurityOperationError(c, err)
				return
			}
			flowPayload.Authorization = middleware.RequireSecurityProof(c, service.VerificationOperation{Scope: service.VerificationScopeAccountBind, Context: context})
			if flowPayload.Authorization == nil {
				return
			}
			flowPayload.SessionIdentity = &identity
		}
		if flowPayload.Telegram != nil {
			if _, _, err := service.ValidateLoginSession(identity); err != nil {
				writeSecurityOperationError(c, err)
				return
			}
			flowPayload.SessionIdentity = &identity
		}
		if request.Intent == model.AuthFlowIntentVerify {
			verification, err := service.StartOAuthVerification(identity, service.VerificationOperation{Scope: request.Scope, Context: request.Context}, request.Provider)
			if err != nil {
				writeSecurityOperationError(c, err)
				return
			}
			flowPayload.Verification = verification
		}
	}
	payload, err := common.Marshal(flowPayload)
	if err != nil {
		writeSecurityOperationError(c, err)
		return
	}
	expiresAt := time.Now().Add(oauthAuthFlowTTL)
	state, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeOAuth,
		Provider:  request.Provider,
		Intent:    request.Intent,
		UserId:    userID,
		SessionId: sessionID,
		Payload:   string(payload),
		ExpiresAt: expiresAt,
	})
	if err != nil {
		writeSecurityOperationError(c, err)
		return
	}
	bindingStarted = request.Intent == model.AuthFlowIntentBind
	data := gin.H{"flow_token": state, "expires_at": expiresAt.Unix()}
	if flowPayload.Telegram != nil {
		data["authorization_url"] = flowPayload.Telegram.AuthorizationURL(state)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

// HandleOAuth handles OAuth callback for all standard OAuth providers
func HandleOAuth(c *gin.Context) {
	providerName := c.Param("provider")
	provider := oauth.GetProvider(providerName)
	if provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgOAuthUnknownProvider),
		})
		return
	}

	// 1. Validate state (CSRF protection)
	state := c.Query("state")
	pendingFlow, err := model.GetAuthFlow(state, model.AuthFlowMatch{
		Purpose:  model.AuthFlowPurposeOAuth,
		Provider: providerName,
	})
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgOAuthStateInvalid),
		})
		return
	}

	consumeMatch := model.AuthFlowMatch{
		Purpose:  model.AuthFlowPurposeOAuth,
		Provider: providerName,
		Intent:   pendingFlow.Intent,
	}
	bindSucceeded, notificationFailed := false, false
	if pendingFlow.Intent == model.AuthFlowIntentBind {
		defer func() {
			recordUserSecurityAudit(c, pendingFlow.UserId, "user.binding_bind", map[string]any{"provider": providerName, "success": bindSucceeded, "notification_failed": notificationFailed})
		}()
	}
	// Bind and verification callbacks must use the dashboard session that started them.
	if pendingFlow.Intent == model.AuthFlowIntentBind || pendingFlow.Intent == model.AuthFlowIntentVerify {
		identity, ok := middleware.GetSessionAuthIdentity(c)
		if !ok || identity.UserID != pendingFlow.UserId || identity.SessionID != pendingFlow.SessionId {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgOAuthStateInvalid),
			})
			return
		}
		consumeMatch.UserId = identity.UserID
		consumeMatch.SessionId = identity.SessionID
		if pendingFlow.Intent == model.AuthFlowIntentBind {
			var payload oauthFlowPayload
			if err := common.UnmarshalJsonStr(pendingFlow.Payload, &payload); err != nil {
				writeSecurityOperationError(c, model.ErrAuthFlowInvalid)
				return
			}
			context, err := common.Marshal(service.AccountBindingContext{Provider: providerName})
			if err != nil {
				writeSecurityOperationError(c, err)
				return
			}
			if err := service.ValidateFlowAuthorization(identity, service.VerificationOperation{Scope: service.VerificationScopeAccountBind, Context: context}, payload.Authorization); err != nil {
				writeSecurityOperationError(c, err)
				return
			}
		}
	} else if pendingFlow.Intent != model.AuthFlowIntentLogin {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	// 3. Check if provider is enabled
	var telegramPayload oauthFlowPayload
	if providerName == "telegram" {
		if err := oauth.TelegramConfigurationError(); err != nil {
			writeSecurityOperationError(c, err)
			return
		}
		if err := common.UnmarshalJsonStr(pendingFlow.Payload, &telegramPayload); err != nil || telegramPayload.Telegram == nil {
			writeSecurityOperationError(c, model.ErrAuthFlowInvalid)
			return
		}
		if pendingFlow.Intent != model.AuthFlowIntentLogin {
			identity, _ := middleware.GetSessionAuthIdentity(c)
			if telegramPayload.SessionIdentity == nil || *telegramPayload.SessionIdentity != identity {
				writeSecurityOperationError(c, model.ErrAuthFlowInvalid)
				return
			}
			if _, _, err := service.ValidateLoginSession(identity); err != nil {
				writeSecurityOperationError(c, err)
				return
			}
		}
		c.Set(oauth.TelegramOAuthFlowContextKey, telegramPayload.Telegram)
	}
	if !provider.IsEnabled() {
		common.ApiErrorI18n(c, i18n.MsgOAuthNotEnabled, providerParams(provider.GetName()))
		return
	}

	// 4. Handle error from provider
	errorCode := c.Query("error")
	if errorCode != "" {
		if _, err := model.ConsumeAuthFlow(state, consumeMatch); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": i18n.T(c, i18n.MsgOAuthStateInvalid)})
			return
		}
		errorDescription := c.Query("error_description")
		if errorDescription == "" {
			errorDescription = errorCode
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": errorDescription,
		})
		return
	}
	// 5. Exchange code for token
	code := c.Query("code")
	token, err := provider.ExchangeToken(c.Request.Context(), code, c)
	if err != nil {
		if providerName == "telegram" {
			writeSecurityOperationError(c, err)
			return
		}
		handleOAuthError(c, err)
		return
	}

	// 6. Get user info
	oauthUser, err := provider.GetUserInfo(c.Request.Context(), token)
	if err != nil {
		if providerName == "telegram" {
			writeSecurityOperationError(c, err)
			return
		}
		handleOAuthError(c, err)
		return
	}
	if pendingFlow.Intent == model.AuthFlowIntentBind {
		bindSucceeded, notificationFailed = handleOAuthBind(c, providerName, provider, oauthUser, pendingFlow, state, consumeMatch)
		return
	}
	flow, err := model.ConsumeAuthFlow(state, consumeMatch)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": i18n.T(c, i18n.MsgOAuthStateInvalid)})
		return
	}

	switch flow.Intent {
	case model.AuthFlowIntentLogin:
		handleOAuthLogin(c, provider, oauthUser, token, flow)
	case model.AuthFlowIntentVerify:
		handleOAuthVerification(c, providerName, oauthUser, flow)
	}
}

func handleOAuthVerification(c *gin.Context, provider string, oauthUser *oauth.OAuthUser, flow *model.AuthFlow) {
	var payload oauthFlowPayload
	if err := common.UnmarshalJsonStr(flow.Payload, &payload); err != nil {
		writeSecurityOperationError(c, err)
		return
	}
	identity, _ := middleware.GetSessionAuthIdentity(c)
	proof, err := service.FinishOAuthVerification(identity, provider, oauthUser.ProviderUserID, payload.Verification)
	if err != nil {
		writeSecurityOperationError(c, err)
		return
	}
	recordUserSecurityAudit(c, identity.UserID, "user.security_verify", map[string]any{"method": proof.Method, "scope": proof.Scope, "provider": provider})
	common.ApiSuccess(c, proof)
}

func handleOAuthLogin(c *gin.Context, provider oauth.Provider, oauthUser *oauth.OAuthUser, token *oauth.OAuthToken, flow *model.AuthFlow) {
	// 7. Find the existing user, or resolve a seed for a new account.
	var payload oauthFlowPayload
	if err := common.UnmarshalJsonStr(flow.Payload, &payload); err != nil {
		writeSecurityOperationError(c, err)
		return
	}
	user, migration, seed, err := findOrCreateOAuthUser(c, provider, oauthUser, token, payload.AffiliateCode)
	if err != nil {
		if errors.Is(err, model.ErrEmailAlreadyTaken) {
			common.ApiErrorI18n(c, i18n.MsgUserEmailAlreadyTaken)
			return
		}
		switch err.(type) {
		case *OAuthUserDeletedError:
			common.ApiErrorI18n(c, i18n.MsgOAuthUserDeleted)
		case *OAuthRegistrationDisabledError:
			common.ApiErrorI18n(c, i18n.MsgUserRegisterDisabled)
		case *OAuthEmailAlreadyTakenError:
			common.ApiErrorI18n(c, i18n.MsgUserEmailAlreadyTaken)
		case *OAuthLegacyBindingNotConfirmedError:
			common.ApiErrorI18n(c, i18n.MsgOAuthNotAutoLinked, providerParams(provider.GetName()))
		default:
			writeSecurityOperationError(c, err)
		}
		return
	}

	// New account. When registration codes are enabled the identity is parked in
	// a short-lived, single-use flow and the caller is sent to the standalone
	// registration-code page; otherwise create and log in right away.
	if seed != nil {
		if common.RegistrationCodeEnabled {
			startOAuthRegistrationFlow(c, flow.Provider, seed)
			return
		}
		created, err := createOAuthUserFromSeed(c, provider, seed, "")
		if err != nil {
			writeOAuthCreateError(c, err)
			return
		}
		setupLogin(created, nil, c)
		return
	}

	// 8. Check user status
	if user.Status != common.UserStatusEnabled {
		common.ApiErrorI18n(c, i18n.MsgOAuthUserBanned)
		return
	}

	// 9. Setup login
	setupLogin(user, migration, c)
}

// startOAuthRegistrationFlow parks a verified provider identity in a single-use,
// short-lived flow and tells the frontend a registration code is still required.
func startOAuthRegistrationFlow(c *gin.Context, providerSlug string, seed *oauthNewUserSeed) {
	payload, err := common.Marshal(seed)
	if err != nil {
		writeSecurityOperationError(c, err)
		return
	}
	expiresAt := time.Now().Add(oauthAuthFlowTTL)
	registerToken, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeOAuthRegister,
		Provider:  providerSlug,
		Intent:    model.AuthFlowIntentLogin,
		Payload:   string(payload),
		ExpiresAt: expiresAt,
	})
	if err != nil {
		writeSecurityOperationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"code":    oauthRegistrationCodeRequiredCode,
		"message": i18n.T(c, i18n.MsgRegistrationCodeRequired),
		"data": gin.H{
			"register_token": registerToken,
			"provider":       providerSlug,
			"expires_at":     expiresAt.Unix(),
		},
	})
}

// CompleteOAuthRegistration finishes a deferred OAuth sign-up: it validates the
// parked identity flow and the supplied registration code, then creates the
// account and issues a session. It runs before login like the OAuth callback.
func CompleteOAuthRegistration(c *gin.Context) {
	if !common.RegisterEnabled {
		common.ApiErrorI18n(c, i18n.MsgUserRegisterDisabled)
		return
	}
	var request struct {
		RegisterToken    string `json:"register_token"`
		RegistrationCode string `json:"registration_code"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	registerToken := strings.TrimSpace(request.RegisterToken)
	if registerToken == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	flow, err := model.GetAuthFlow(registerToken, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuthRegister})
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": i18n.T(c, i18n.MsgOAuthStateInvalid)})
		return
	}
	provider := oauth.GetProvider(flow.Provider)
	if provider == nil || !provider.IsEnabled() {
		common.ApiErrorI18n(c, i18n.MsgOAuthNotEnabled, providerParams(flow.Provider))
		return
	}
	var seed oauthNewUserSeed
	if err := common.UnmarshalJsonStr(flow.Payload, &seed); err != nil || seed.ProviderUserID == "" {
		writeSecurityOperationError(c, model.ErrAuthFlowInvalid)
		return
	}

	// Validate the code without consuming it, so a wrong code lets the user retry
	// on the same page without burning the parked flow. If the operator turned the
	// gate off in the meantime, just finish the sign-up.
	regCode := ""
	if common.RegistrationCodeEnabled {
		regCode = model.NormalizeRegistrationCodeValue(request.RegistrationCode)
		if regCode == "" {
			common.ApiErrorI18n(c, i18n.MsgRegistrationCodeRequired)
			return
		}
		if err := model.CheckRegistrationCodeUsable(regCode); err != nil {
			common.ApiErrorI18n(c, i18n.MsgRegistrationCodeInvalid)
			return
		}
	}

	// Guard against a duplicate account when the same identity completes twice
	// (e.g. two tabs); send them back through login instead.
	if provider.IsUserIDTaken(seed.ProviderUserID) {
		common.ApiErrorI18n(c, i18n.MsgOAuthAlreadyBound, providerParams(provider.GetName()))
		return
	}

	// Consume the parked flow once; replay/expiry is rejected atomically.
	if _, err := model.ConsumeAuthFlow(registerToken, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuthRegister}); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": i18n.T(c, i18n.MsgOAuthStateInvalid)})
		return
	}

	user, err := createOAuthUserFromSeed(c, provider, &seed, regCode)
	if err != nil {
		writeOAuthCreateError(c, err)
		return
	}
	setupLogin(user, nil, c)
}

// writeOAuthCreateError maps the errors createOAuthUserFromSeed can return to a
// translated response. Shared by the callback (no-code) path and the completion
// endpoint.
func writeOAuthCreateError(c *gin.Context, err error) {
	if errors.Is(err, model.ErrEmailAlreadyTaken) {
		common.ApiErrorI18n(c, i18n.MsgUserEmailAlreadyTaken)
		return
	}
	switch err.(type) {
	case *OAuthEmailAlreadyTakenError:
		common.ApiErrorI18n(c, i18n.MsgUserEmailAlreadyTaken)
	case *OAuthRegistrationCodeInvalidError:
		common.ApiErrorI18n(c, i18n.MsgRegistrationCodeInvalid)
	default:
		writeSecurityOperationError(c, err)
	}
}

// handleOAuthBind handles binding OAuth account to existing user
func handleOAuthBind(c *gin.Context, providerName string, provider oauth.Provider, oauthUser *oauth.OAuthUser, flow *model.AuthFlow, state string, match model.AuthFlowMatch) (bool, bool) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		writeSecurityOperationError(c, service.ErrAuthTokenInvalid)
		return false, false
	}
	var payload oauthFlowPayload
	if err := common.UnmarshalJsonStr(flow.Payload, &payload); err != nil {
		writeSecurityOperationError(c, model.ErrAuthFlowInvalid)
		return false, false
	}
	context, err := common.Marshal(service.AccountBindingContext{Provider: providerName})
	if err != nil {
		writeSecurityOperationError(c, err)
		return false, false
	}
	// Recheck after the external provider round trip, then validate the session
	// under the transaction's locks before consuming the flow and writing.
	if err := service.ValidateFlowAuthorization(identity, service.VerificationOperation{Scope: service.VerificationScopeAccountBind, Context: context}, payload.Authorization); err != nil {
		writeSecurityOperationError(c, err)
		return false, false
	}
	if provider.IsUserIDTaken(oauthUser.ProviderUserID) {
		common.ApiErrorI18n(c, i18n.MsgOAuthAlreadyBound, providerParams(provider.GetName()))
		return false, false
	}
	_, err = model.ConsumeAuthFlowWithAction(state, match, func(tx *gorm.DB, _ *model.AuthFlow) error {
		if providerName == "telegram" {
			return model.BindTelegramForSessionWithTx(tx, identity, oauthUser.ProviderUserID)
		}
		if custom, ok := provider.(*oauth.GenericOAuthProvider); ok {
			return model.UpdateUserOAuthBindingForSessionWithTx(tx, identity, custom.GetProviderId(), oauthUser.ProviderUserID)
		}
		return model.UpdateUserBindColumnForSessionWithTx(tx, identity, provider.ProviderUserIDColumn(), oauthUser.ProviderUserID)
	})
	if err != nil {
		writeSecurityOperationError(c, err)
		return false, false
	}
	user, err := model.GetUserById(identity.UserID, false)
	if err != nil {
		writeSecurityOperationError(c, err)
		return true, true
	}
	notificationFailed := service.NotifyAccountSecurityChange(user.Email, "Login account linked: "+provider.GetName()) != nil
	common.ApiSuccessI18n(c, i18n.MsgOAuthBindSuccess, gin.H{"action": "bind", "notification_warning": notificationFailed})
	return true, notificationFailed
}

// findOrCreateOAuthUser resolves the existing user for a provider identity. It
// never creates the account: when the identity is new it returns an
// oauthNewUserSeed so the caller can decide whether a registration code is
// required first. For a legacy GitHub binding still awaiting login verification
// it also returns the rewrite to carry into the challenge.
func findOrCreateOAuthUser(c *gin.Context, provider oauth.Provider, oauthUser *oauth.OAuthUser, token *oauth.OAuthToken, affiliateCode string) (*model.User, *service.LegacyGitHubMigration, *oauthNewUserSeed, error) {
	user := &model.User{}
	if provider.ProviderUserIDColumn() == "telegram_id" {
		err := provider.FillUserByProviderID(user, oauthUser.ProviderUserID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, oauth.ErrTelegramAccountNotBound
		}
		return user, nil, nil, err
	}

	// Check if user already exists with new ID
	if provider.IsUserIDTaken(oauthUser.ProviderUserID) {
		err := provider.FillUserByProviderID(user, oauthUser.ProviderUserID)
		if err != nil {
			return nil, nil, nil, err
		}
		// Check if user has been deleted
		if user.Id == 0 {
			return nil, nil, nil, &OAuthUserDeletedError{}
		}
		return user, nil, nil, nil
	}

	// Legacy GitHub bindings stored the login name, which only points at a
	// candidate account. Values that are all digits are already numeric account
	// IDs and never take part in the comparison.
	legacyID, _ := oauthUser.Extra["legacy_id"].(string)
	if strings.ContainsFunc(legacyID, func(r rune) bool { return r < '0' || r > '9' }) && provider.IsUserIDTaken(legacyID) {
		if err := provider.FillUserByProviderID(user, legacyID); err != nil {
			return nil, nil, nil, err
		}
		if user.Id != 0 {
			state, err := model.GetUserVerificationState(user.Id)
			if err != nil {
				return nil, nil, nil, err
			}
			if state.HasTwoFA || state.HasPasskey {
				// The rewrite is written in the transaction that issues the session
				// once the login verification completes.
				return user, &service.LegacyGitHubMigration{GitHubID: oauthUser.ProviderUserID, LegacyID: legacyID}, nil, nil
			}
			// Without a second factor, one of the addresses the provider has
			// confirmed must match the account email. The list is fetched only here
			// and never recorded.
			reason, matched := "no_matching_evidence", false
			if emailProvider, ok := provider.(oauth.VerifiedEmailProvider); ok && user.Email != "" {
				emails, err := emailProvider.GetVerifiedEmails(c.Request.Context(), token)
				if err != nil {
					common.SysError(fmt.Sprintf("[OAuth] Failed to load verified emails for user %d: %s", user.Id, err.Error()))
					reason = "verified_emails_unavailable"
				}
				accountEmail := model.NormalizeEmail(user.Email)
				matched = slices.ContainsFunc(emails, func(email string) bool { return model.NormalizeEmail(email) == accountEmail })
			}
			if !matched {
				recordLegacyGitHubBindingAudit(c, user, false, map[string]any{"legacy_id": legacyID, "provider_user_id": oauthUser.ProviderUserID, "reason": reason})
				return nil, nil, nil, &OAuthLegacyBindingNotConfirmedError{}
			}
			written := false
			err = model.DB.Transaction(func(tx *gorm.DB) error {
				var err error
				written, err = model.MigrateLegacyGitHubBindingWithTx(tx, user.Id, legacyID, oauthUser.ProviderUserID)
				return err
			})
			if err != nil {
				return nil, nil, nil, err
			}
			if written {
				user.GitHubId = oauthUser.ProviderUserID
				notificationFailed := service.NotifyAccountSecurityChange(user.Email, "Login account linked: "+provider.GetName()) != nil
				recordLegacyGitHubBindingAudit(c, user, true, map[string]any{
					"legacy_id": legacyID, "provider_user_id": oauthUser.ProviderUserID,
					"verified_email_matched": true, "notification_failed": notificationFailed,
				})
			}
			return user, nil, nil, nil
		}
	}

	// User doesn't exist. Registration must be enabled to proceed; the actual
	// account creation is deferred to createOAuthUserFromSeed so a registration
	// code can be collected first when required.
	if !common.RegisterEnabled {
		return nil, nil, nil, &OAuthRegistrationDisabledError{}
	}
	return nil, nil, &oauthNewUserSeed{
		ProviderUserID: oauthUser.ProviderUserID,
		Username:       oauthUser.Username,
		DisplayName:    oauthUser.DisplayName,
		Email:          oauthUser.Email,
		AffiliateCode:  affiliateCode,
	}, nil
}

// createOAuthUserFromSeed turns a verified provider identity into an account. It
// runs on the callback path when no registration code is required, and from
// CompleteOAuthRegistration once the user has supplied a valid one. When
// registration codes are enabled the code is consumed atomically before the
// account is written; a create failure afterwards records a compensation audit.
func createOAuthUserFromSeed(c *gin.Context, provider oauth.Provider, seed *oauthNewUserSeed, registrationCode string) (*model.User, error) {
	user := &model.User{}
	user.Username = provider.GetProviderPrefix() + strconv.Itoa(model.GetMaxUserId()+1)
	if seed.Username != "" {
		if exists, err := model.CheckUserExistOrDeleted(seed.Username, ""); err == nil && !exists {
			// 防止索引退化
			if len(seed.Username) <= model.UserNameMaxLength {
				user.Username = seed.Username
			}
		}
	}

	if seed.DisplayName != "" {
		user.DisplayName = seed.DisplayName
	} else if seed.Username != "" {
		user.DisplayName = seed.Username
	} else {
		user.DisplayName = provider.GetName() + " User"
	}
	if seed.Email != "" {
		user.Email = model.NormalizeEmail(seed.Email)
		if err := model.EnsureEmailAvailable(user.Email, 0); err != nil {
			if errors.Is(err, model.ErrEmailAlreadyTaken) {
				return nil, &OAuthEmailAlreadyTakenError{}
			}
			return nil, err
		}
	}
	user.Role = common.RoleCommonUser
	user.Status = common.UserStatusEnabled

	// Handle affiliate code
	inviterId := 0
	if seed.AffiliateCode != "" {
		inviterId, _ = model.GetUserIdByAffCode(seed.AffiliateCode)
	}

	// 注册码开启时，在建号前原子消费。OAuth 是多跳流程；若消费成功但建号失败，
	// 写补偿审计供运维对账。
	consumedRcID := 0
	if common.RegistrationCodeEnabled {
		rcID, _, err := model.ConsumeRegistrationCode(registrationCode)
		if err != nil {
			return nil, &OAuthRegistrationCodeInvalidError{err}
		}
		consumedRcID = rcID
	}

	// Use transaction to ensure user creation and OAuth binding are atomic
	if genericProvider, ok := provider.(*oauth.GenericOAuthProvider); ok {
		// Custom provider: create user and binding in a transaction
		err := model.DB.Transaction(func(tx *gorm.DB) error {
			// Create user
			if err := user.InsertWithTx(tx, inviterId); err != nil {
				return err
			}

			// Create OAuth binding
			binding := &model.UserOAuthBinding{
				UserId:         user.Id,
				ProviderId:     genericProvider.GetProviderId(),
				ProviderUserId: seed.ProviderUserID,
			}
			return model.CreateUserOAuthBindingWithTx(tx, binding)
		})
		if err != nil {
			if consumedRcID > 0 {
				recordOAuthRegistrationCodeCompensation(c, consumedRcID, "oauth_user_create_failed")
			}
			return nil, err
		}

		// Perform post-transaction tasks (logs, sidebar config, inviter rewards)
		user.FinalizeOAuthUserCreation(inviterId)
	} else {
		// Built-in provider: create user and update provider ID in a transaction
		err := model.DB.Transaction(func(tx *gorm.DB) error {
			// Create user
			if err := user.InsertWithTx(tx, inviterId); err != nil {
				return err
			}

			// Set the provider user ID on the user model and update
			provider.SetProviderUserID(user, seed.ProviderUserID)
			return tx.Model(user).Updates(map[string]any{
				"github_id":   user.GitHubId,
				"discord_id":  user.DiscordId,
				"oidc_id":     user.OidcId,
				"linux_do_id": user.LinuxDOId,
				"wechat_id":   user.WeChatId,
				"telegram_id": user.TelegramId,
			}).Error
		})
		if err != nil {
			if consumedRcID > 0 {
				recordOAuthRegistrationCodeCompensation(c, consumedRcID, "oauth_user_create_failed")
			}
			return nil, err
		}

		// Perform post-transaction tasks
		user.FinalizeOAuthUserCreation(inviterId)
	}

	return user, nil
}

// recordLegacyGitHubBindingAudit records the outcome of a legacy GitHub binding
// rewrite as an account binding event. No session exists yet on the login path,
// so the row carries the user's own role instead of the request context.
func recordLegacyGitHubBindingAudit(c *gin.Context, user *model.User, success bool, params map[string]any) {
	params["provider"], params["legacy_migration"], params["success"] = "github", true, success
	model.RecordOperationAuditLog(user.Id, user.Role, auditContentEN("user.binding_bind", params), c.ClientIP(), "user.binding_bind", params, nil, &model.AuditRequestInfo{
		Method: c.Request.Method, Route: c.FullPath(), Path: c.FullPath(), Status: c.Writer.Status(), Success: success,
	}, c)
}

// Error types for OAuth
type OAuthUserDeletedError struct{}

func (e *OAuthUserDeletedError) Error() string {
	return "user has been deleted"
}

type OAuthRegistrationDisabledError struct{}

func (e *OAuthRegistrationDisabledError) Error() string {
	return "registration is disabled"
}

type OAuthRegistrationCodeInvalidError struct {
	Err error
}

func (e *OAuthRegistrationCodeInvalidError) Error() string {
	return "invalid registration code: " + e.Err.Error()
}

func (e *OAuthRegistrationCodeInvalidError) Unwrap() error { return e.Err }

// recordOAuthRegistrationCodeCompensation 记录 OAuth 建号失败时已消费注册码的补偿审计，
// 供运维对账。不含码值，只含注册码 ID。
func recordOAuthRegistrationCodeCompensation(c *gin.Context, rcID int, reason string) {
	model.RecordOperationAuditLog(c.GetInt("id"), c.GetInt("role"), auditContentEN("registration_code.consume_failed_after_oauth_create", map[string]any{
		"registration_code_id": rcID,
		"reason":               reason,
		"success":              false,
	}), c.ClientIP(), "registration_code.consume_failed_after_oauth_create", map[string]any{
		"registration_code_id": rcID,
		"reason":               reason,
	}, nil, &model.AuditRequestInfo{
		Method: c.Request.Method, Route: c.FullPath(), Path: c.FullPath(), Status: c.Writer.Status(), Success: false,
	}, c)
}

type OAuthEmailAlreadyTakenError struct{}

func (e *OAuthEmailAlreadyTakenError) Error() string {
	return "email is already in use"
}

// OAuthLegacyBindingNotConfirmedError reports a legacy GitHub binding match that
// neither a second factor nor a confirmed provider email backed.
type OAuthLegacyBindingNotConfirmedError struct{}

func (e *OAuthLegacyBindingNotConfirmedError) Error() string {
	return "legacy binding was not confirmed"
}

// handleOAuthError handles OAuth errors and returns translated message
func handleOAuthError(c *gin.Context, err error) {
	switch e := err.(type) {
	case *oauth.OAuthError:
		if e.Params != nil {
			common.ApiErrorI18n(c, e.MsgKey, e.Params)
		} else {
			common.ApiErrorI18n(c, e.MsgKey)
		}
	case *oauth.AccessDeniedError:
		common.ApiErrorMsg(c, e.Message)
	case *oauth.TrustLevelError:
		common.ApiErrorI18n(c, i18n.MsgOAuthTrustLevelLow)
	default:
		writeSecurityOperationError(c, err)
	}
}
