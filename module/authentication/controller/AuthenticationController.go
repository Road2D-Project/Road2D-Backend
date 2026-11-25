package controller

import (
	"Road-To-Destination-BE/middleware"
	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/authentication/model/request"
	"Road-To-Destination-BE/module/authentication/model/response"
	"Road-To-Destination-BE/module/authentication/repository"
	"Road-To-Destination-BE/module/authentication/service"
	"Road-To-Destination-BE/module/mail"
	"Road-To-Destination-BE/module/objectStorage"
	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/utils/customValidator"
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var _ share.RouterRegistrar = (*AuthenticationController)(nil)

// Keep swagger types in this file so swag can resolve them.
var (
	_ = model.User{}
	_ = request.RegisterRequest{}
	_ = response.RegisterResponse{}
	_ = request.LoginRequest{}
	_ = response.LoginEnvelope{}
	_ = request.RefreshRequest{}
	_ = response.RefreshResponse{}
	_ = request.RefreshRequest{}
	_ = response.LogoutResponse{}
	_ = request.ForgetPasswordRequest{}
	_ = response.ForgetPasswordResponse{}
	_ = request.ResetPasswordRequest{}
	_ = response.ResetPasswordResponse{}
	_ = request.ConfirmAvatarRequest{}
	_ = response.AvatarUploadResponse{}
	_ = response.AvatarURLResponse{}
	_ = share.ErrorResponse{}
)

type AuthenticationController struct {
	db          *gorm.DB
	redisClient *redis.Client
	validator   *validator.Validate
	rateLimiter middleware.RateLimiter
	storage     *objectStorage.Client
}

func NewAuthenticationController(db *gorm.DB, client *redis.Client, validator *validator.Validate, rateLimiter middleware.RateLimiter, storage *objectStorage.Client) *AuthenticationController {
	return &AuthenticationController{db: db, redisClient: client, validator: validator, rateLimiter: rateLimiter, storage: storage}
}

func (ctrl AuthenticationController) RegisterRoutes(router *gin.RouterGroup) {
	nodeGroup := router.Group("/auth")
	{
		auth := middleware.NewAuthenticationMiddleware(repository.NewUserRepository(ctrl.db))
		nodeGroup.POST("/user/register", ctrl.rateLimiter.RequireIPLimiter(), ctrl.HandleRegister())
		nodeGroup.POST("/user/login", ctrl.rateLimiter.RequireIPLimiter(), ctrl.HandleLogin())
		nodeGroup.GET("/user/me", auth.RequireAuth(), ctrl.HandleMyProfile())
		nodeGroup.POST("/user/avatar", auth.RequireAuth(), ctrl.HandleRequestAvatar())
		nodeGroup.POST("/user/avatar/confirm", auth.RequireAuth(), ctrl.HandleConfirmAvatar())
		nodeGroup.DELETE("/user/avatar", auth.RequireAuth(), ctrl.HandleDeleteAvatar())
		nodeGroup.GET("/user/:id", auth.RequireAuth(), ctrl.HandleGetUserProfile())
		nodeGroup.POST("/user/refresh", ctrl.rateLimiter.RequireIPLimiter(), ctrl.HandleRefreshToken())
		nodeGroup.POST("/user/logout", ctrl.rateLimiter.RequireIPLimiter(), ctrl.HandleLogout())
		nodeGroup.POST("/user/forget-password", ctrl.rateLimiter.RequireIPLimiter(), ctrl.HandleForgetPassword())
		nodeGroup.GET("/user/reset-password/:resetToken", ctrl.rateLimiter.RequireIPLimiter(), ctrl.HandleResetPasswordPage())
		nodeGroup.POST("/user/reset-password/:resetToken", ctrl.rateLimiter.RequireIPLimiter(), ctrl.HandleResetPassword())
	}
}

// HandleRegister godoc
// @Summary      Register
// @Description  Create a user account. Returns the new user UUID.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      request.RegisterRequest  true  "Register payload"
// @Success      200   {object}  response.RegisterResponse
// @Failure      400   {object}  share.ErrorResponse
// @Router       /auth/user/register [post]
func (ctrl AuthenticationController) HandleRegister() gin.HandlerFunc {
	return func(c *gin.Context) {
		var registerRequest request.RegisterRequest
		if err := c.ShouldBind(&registerRequest); err != nil {
			jsonBindError(c, err)
			return
		}
		err := ctrl.validator.Struct(registerRequest)
		if err != nil {
			jsonBindError(c, err)
			return
		}
		if !ctrl.allowAuthAccount(c, "register", registerRequest.Username) {
			return
		}
		repo := repository.NewUserRepository(ctrl.db)
		registerService := service.NewRegisterService(repo)
		userID, err := registerService.Register(registerRequest, c.Request.Context())
		if err != nil {
			jsonError(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, response.RegisterResponse{UserID: userID})
	}
}

// HandleLogin godoc
// @Summary      Login
// @Description  Authenticate with username and password. Returns access and refresh tokens.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      request.LoginRequest  true  "Login payload"
// @Success      200   {object}  response.LoginEnvelope
// @Failure      400   {object}  share.ErrorResponse
// @Failure      401   {object}  share.ErrorResponse
// @Failure      429   {object}  share.ErrorResponse
// @Failure      503   {object}  share.ErrorResponse
// @Router       /auth/user/login [post]
func (ctrl AuthenticationController) HandleLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		var loginRequest request.LoginRequest
		if err := c.ShouldBind(&loginRequest); err != nil {
			jsonBindError(c, err)
			return
		}
		if !ctrl.allowAuthAccount(c, "login", loginRequest.Username) {
			return
		}
		repo := repository.NewCacheUserRepository(ctrl.db, ctrl.redisClient)
		loginService := service.NewLoginService(repo)
		accessToken, refreshToken, err := loginService.Login(c.Request.Context(), loginRequest)
		if err != nil {
			status := http.StatusUnauthorized
			if errors.Is(err, service.ErrAuthUnavailable) || errors.Is(err, service.ErrJWTConfiguration) {
				status = http.StatusServiceUnavailable
			}
			jsonError(c, status, http.StatusText(status))
			return
		}
		c.JSON(http.StatusOK, response.LoginEnvelope{
			Data: response.LoginResponse{AccessToken: accessToken, RefreshToken: refreshToken},
		})
	}
}

// HandleGetUserProfile godoc
// @Summary      Get user profile
// @Description  Return a user by UUID. Password hash is never included.
// @Tags         auth
// @Produce      json
// @Param        id   path      string  true  "User UUID"  format(uuid)
// @Success      200  {object}  model.User
// @Failure      400  {object}  share.ErrorResponse
// @Failure      404  {object}  share.ErrorResponse
// @Router       /auth/user/{id} [get]
func (ctrl AuthenticationController) HandleGetUserProfile() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			jsonError(c, http.StatusBadRequest, "invalid user id")
			return
		}
		repo := repository.NewUserRepository(ctrl.db)
		user, err := repo.FindUserByID(c.Request.Context(), id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				jsonError(c, http.StatusNotFound, "user not found")
				return
			}
			jsonError(c, http.StatusBadRequest, err.Error())
			return
		}
		isOwner := user.Username == middleware.GetCurrentUsername(c)
		if err := ctrl.resolveAvatarURL(c, user); err != nil {
			mapImageError(c, err)
			return
		}
		c.Header("Cache-Control", "private, no-store")
		c.JSON(http.StatusOK, response.UserToUserProfileResponse(user, isOwner))
	}
}

// HandleRefreshToken godoc
// @Summary      Refresh access token
// @Description  Issue a new access token after checking the refresh token, logout revocation and current database session version. Password resets invalidate older sessions.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      request.RefreshRequest  true  "Refresh payload"
// @Success      200   {object}  response.RefreshResponse
// @Failure      400   {object}  share.ErrorResponse
// @Failure      401   {object}  share.ErrorResponse
// @Failure      429   {object}  share.ErrorResponse
// @Failure      503   {object}  share.ErrorResponse
// @Router       /auth/user/refresh [post]
func (ctrl AuthenticationController) HandleRefreshToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request request.RefreshRequest
		if err := c.ShouldBind(&request); err != nil {
			jsonBindError(c, err)
			return
		}
		logoutService := ctrl.logoutService()
		user, err := logoutService.AssertRefreshUsable(c.Request.Context(), request.RefreshToken)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, service.ErrRefreshRevoked) || errors.Is(err, service.ErrTokenExpired) ||
				errors.Is(err, service.ErrTokenInvalid) || errors.Is(err, service.ErrSessionRevoked) {
				status = http.StatusUnauthorized
			}
			if errors.Is(err, service.ErrAuthUnavailable) || errors.Is(err, service.ErrJWTConfiguration) {
				status = http.StatusServiceUnavailable
			}
			jsonError(c, status, err.Error())
			return
		}
		if !ctrl.allowAuthAccount(c, "refresh", user.ID.String()) {
			return
		}
		accessToken, err := service.CreateToken(user)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, response.RefreshResponse{AccessToken: accessToken})
	}
}

// HandleLogout godoc
// @Summary      Logout
// @Description  Revoke the presented refresh token. The SHA-256 hash is stored in Redis with TTL until JWT exp, and persisted in Postgres so a Redis restart cannot revive the session.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      request.LogoutRequest  true  "Refresh token to revoke"
// @Success      200   {object}  response.LogoutResponse
// @Failure      400   {object}  share.ErrorResponse
// @Failure      503   {object}  share.ErrorResponse
// @Router       /auth/user/logout [post]
func (ctrl AuthenticationController) HandleLogout() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body request.LogoutRequest
		if err := c.ShouldBind(&body); err != nil {
			jsonBindError(c, err)
			return
		}
		logoutService := ctrl.logoutService()
		if err := logoutService.Logout(c.Request.Context(), body.RefreshToken); err != nil {
			if errors.Is(err, repository.ErrRevokeStoreUnavailable) || errors.Is(err, service.ErrJWTConfiguration) {
				jsonError(c, http.StatusServiceUnavailable, err.Error())
				return
			}
			jsonError(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, response.LogoutResponse{Message: "logged out"})
	}
}

func (ctrl AuthenticationController) logoutService() *service.LogoutService {
	return service.NewLogoutService(repository.NewRefreshRevokeStore(ctrl.redisClient, ctrl.db), repository.NewUserRepository(ctrl.db))
}

// HandleForgetPassword godoc
// @Summary      Forget password
// @Description  If the email exists, send a reset-password HTML mail with a one-time link.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      request.ForgetPasswordRequest  true  "Forget password payload"
// @Success      200   {object}  response.ForgetPasswordResponse
// @Failure      400   {object}  share.ErrorResponse
// @Router       /auth/user/forget-password [post]
func (ctrl AuthenticationController) HandleForgetPassword() gin.HandlerFunc {
	return func(c *gin.Context) {
		var emailRequest request.ForgetPasswordRequest
		if err := c.ShouldBind(&emailRequest); err != nil {
			jsonBindError(c, err)
			return
		}
		if !ctrl.allowAuthAccount(c, "forget-password", emailRequest.Email) {
			return
		}
		userRepo := repository.NewUserRepository(ctrl.db)
		forgetPasswordService := service.NewForgetPasswordService(userRepo, mail.NewFromEnv(), repository.NewPasswordResetStore(ctrl.redisClient))
		message, err := forgetPasswordService.ForgetPassword(c.Request.Context(), emailRequest)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, message)
	}
}

func (ctrl AuthenticationController) HandleResetPasswordPage() gin.HandlerFunc {
	return func(c *gin.Context) {
		resetToken := c.Param("resetToken")
		page, status := resetPasswordPage(c.Request.Context(), repository.NewPasswordResetStore(ctrl.redisClient), resetToken)
		c.Header("Cache-Control", "no-store")
		c.Data(status, "text/html; charset=utf-8", []byte(page))
	}
}

// HandleResetPassword godoc
// @Summary      Reset password
// @Description  Set a new password and revoke older access, refresh and reset tokens by atomically advancing the user's session version.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        resetToken  path      string                        true  "Reset JWT from mail"
// @Param        body        body      request.ResetPasswordRequest  true  "New password"
// @Success      200         {object}  response.ResetPasswordResponse
// @Failure      400         {object}  share.ErrorResponse
// @Router       /auth/user/reset-password/{resetToken} [post]
func (ctrl AuthenticationController) HandleResetPassword() gin.HandlerFunc {
	return func(c *gin.Context) {
		resetToken := c.Param("resetToken")
		claims, err := service.VerifyResetPasswordToken(resetToken)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err.Error())
			return
		}
		userId, _ := uuid.Parse(claims["sub"].(string))
		email, _ := claims["email"].(string)
		version, _ := service.ClaimsSessionVersion(claims)
		if !ctrl.allowAuthAccount(c, "reset-password", userId.String()) {
			return
		}
		var resetPasswordRequest request.ResetPasswordRequest
		if err := c.ShouldBind(&resetPasswordRequest); err != nil {
			jsonBindError(c, err)
			return
		}
		err = ctrl.validator.Struct(resetPasswordRequest)
		if err != nil {
			jsonBindError(c, err)
			return
		}
		resetPasswordRepo := repository.NewCacheUserRepository(ctrl.db, ctrl.redisClient)
		resetPasswordService := service.NewResetPasswordService(resetPasswordRepo, repository.NewPasswordResetStore(ctrl.redisClient))
		msg, err := resetPasswordService.ResetPassword(c.Request.Context(), userId, email, resetPasswordRequest.Password, resetPasswordRequest.ConfirmPassword, version)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, response.ResetPasswordResponse{Message: msg})
	}
}

// HandleMyProfile godoc
// @Summary      My profile
// @Description  Return the authenticated user. Password hash is never included.
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  model.User
// @Failure      401  {object}  share.ErrorResponse
// @Router       /auth/user/me [get]
func (ctrl AuthenticationController) HandleMyProfile() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := middleware.GetCurrentUser(c)
		if user == nil {
			jsonError(c, http.StatusUnauthorized, "Unauthorized")
			return
		}
		// Keep the authenticated/cached user record free of expiring URLs.
		view := *user
		if err := ctrl.resolveAvatarURL(c, &view); err != nil {
			mapImageError(c, err)
			return
		}
		c.Header("Cache-Control", "private, no-store")
		c.JSON(http.StatusOK, &view)
	}
}

func resetPasswordPage(ctx context.Context, store *repository.PasswordResetStore, resetToken string) (string, int) {
	userId, _, username, err := parseResetToken(resetToken)
	if err != nil {
		return invalidResetPage("Token đã hết hạn hoặc không hợp lệ.")
	}
	wait, blocked, err := store.ResetCooldownRemaining(ctx, userId)
	if err != nil {
		return invalidResetPage(err.Error())
	}
	if blocked {
		return invalidResetPage((&repository.PasswordResetCooldownError{WaitMinutes: wait}).Error())
	}
	html, renderErr := mail.Render(mail.KindResetPasswordPage, mail.ResetPasswordPageForm{
		Valid:    true,
		Username: username,
	})
	if renderErr != nil {
		return renderErr.Error(), http.StatusInternalServerError
	}
	return html, http.StatusOK
}

func invalidResetPage(message string) (string, int) {
	html, renderErr := mail.Render(mail.KindResetPasswordPage, mail.ResetPasswordPageForm{
		Valid: false,
		Error: message,
	})
	if renderErr != nil {
		return renderErr.Error(), http.StatusInternalServerError
	}
	return html, http.StatusBadRequest
}

func parseResetToken(resetToken string) (uuid.UUID, string, string, error) {
	claims, err := service.VerifyResetPasswordToken(resetToken)
	if err != nil {
		return uuid.Nil, "", "", errors.New("invalid token")
	}
	idValue, ok := claims["id"].(string)
	if !ok || idValue == "" {
		return uuid.Nil, "", "", errors.New("user not found")
	}
	userId, err := uuid.Parse(idValue)
	if err != nil {
		return uuid.Nil, "", "", errors.New("user not found")
	}
	email, _ := claims["email"].(string)
	username, _ := claims["username"].(string)
	return userId, email, username, nil
}

func jsonError(c *gin.Context, status int, message string) {
	c.JSON(status, share.NewError(status, message))
}

func jsonBindError(c *gin.Context, err error) {
	body := customValidator.HandleValidationError(err)
	c.JSON(body.Status, body)
}
