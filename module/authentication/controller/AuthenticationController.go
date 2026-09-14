package controller

import (
	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/authentication/model/request"
	"Road-To-Destination-BE/module/authentication/model/response"
	"Road-To-Destination-BE/module/authentication/repository"
	"Road-To-Destination-BE/module/authentication/service"
	"Road-To-Destination-BE/module/mail"
	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/utils/customValidator"
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
	_ = request.ForgetPasswordRequest{}
	_ = response.ForgetPasswordResponse{}
	_ = request.ResetPasswordRequest{}
	_ = share.ErrorResponse{}
)

type AuthenticationController struct {
	db          *gorm.DB
	redisClient *redis.Client
	validator   *validator.Validate
}

func NewAuthenticationController(db *gorm.DB, client *redis.Client, validator *validator.Validate) *AuthenticationController {
	return &AuthenticationController{db: db, redisClient: client, validator: validator}
}

func (ctrl AuthenticationController) RegisterRoutes(router *gin.RouterGroup) {
	nodeGroup := router.Group("/auth")
	{
		nodeGroup.POST("/user/register", ctrl.HandleRegister())
		nodeGroup.POST("/user/login", ctrl.HandleLogin())
		nodeGroup.GET("/user/:id", ctrl.HandleGetUserProfile())
		nodeGroup.POST("/user/refresh", ctrl.HandleRefreshToken())
		nodeGroup.POST("/user/forget-password", ctrl.HandleForgetPassword())
		nodeGroup.GET("/user/reset-password/:resetToken", ctrl.HandleResetPasswordPage())
		nodeGroup.POST("/user/reset-password/:resetToken", ctrl.HandleResetPassword())
	}
}

// HandleRegister godoc
// @Summary      Register
// @Description  Create a user account. Returns the new user UUID.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      model.RegisterRequest  true  "Register payload"
// @Success      200   {object}  model.RegisterResponse
// @Failure      400   {object}  share.ErrorResponse
// @Router       /auth/user/register [post]
func (ctrl AuthenticationController) HandleRegister() gin.HandlerFunc {
	return func(c *gin.Context) {
		var registerRequest request.RegisterRequest
		if err := c.ShouldBind(&registerRequest); err != nil {
			c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: err.Error()})
			return
		}
		repo := repository.NewUserRepository(ctrl.db)
		registerService := service.NewRegisterService(repo)
		userID, err := registerService.Register(registerRequest, c.Request.Context())
		if err != nil {
			c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: err.Error()})
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
// @Param        body  body      model.LoginRequest  true  "Login payload"
// @Success      200   {object}  model.LoginEnvelope
// @Failure      400   {object}  share.ErrorResponse
// @Router       /auth/user/login [post]
func (ctrl AuthenticationController) HandleLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		var loginRequest request.LoginRequest
		if err := c.ShouldBind(&loginRequest); err != nil {
			c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: err.Error()})
			return
		}
		repo := repository.NewCacheUserRepository(ctrl.db, ctrl.redisClient)
		loginService := service.NewLoginService(repo)
		accessToken, refreshToken, err := loginService.Login(c.Request.Context(), loginRequest)
		if err != nil {
			c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: err.Error()})
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
			c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: "invalid user id"})
			return
		}
		repo := repository.NewUserRepository(ctrl.db)
		user, err := repo.FindUserByID(c.Request.Context(), id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, share.ErrorResponse{Error: "user not found"})
				return
			}
			c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusOK, user)
	}
}

// HandleRefreshToken godoc
// @Summary      Refresh access token
// @Description  Issue a new access token from a valid refresh token.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      model.RefreshRequest  true  "Refresh payload"
// @Success      200   {object}  model.RefreshResponse
// @Failure      400   {object}  share.ErrorResponse
// @Router       /auth/user/refresh [post]
func (ctrl AuthenticationController) HandleRefreshToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request request.RefreshRequest
		if err := c.ShouldBind(&request); err != nil {
			c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: err.Error()})
			return
		}
		claims, err := service.VerifyRefreshToken(request.RefreshToken)
		if err != nil {
			c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: err.Error()})
			return
		}
		username, ok := claims["username"].(string)
		if !ok || username == "" {
			c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: "invalid token payload"})
			return
		}
		accessToken, err := service.CreateToken(username)
		if err != nil {
			c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusOK, response.RefreshResponse{AccessToken: accessToken})
	}
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
		var passwordRequest request.ForgetPasswordRequest
		if err := c.ShouldBind(&passwordRequest); err != nil {
			c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: err.Error()})
			return
		}
		userRepo := repository.NewUserRepository(ctrl.db)
		forgetPasswordService := service.NewForgetPasswordService(userRepo, mail.NewFromEnv())
		message, err := forgetPasswordService.ForgetPassword(c.Request.Context(), passwordRequest)
		if err != nil {
			c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": message})
	}
}

func (ctrl AuthenticationController) HandleResetPasswordPage() gin.HandlerFunc {
	return func(c *gin.Context) {
		resetToken := c.Param("resetToken")
		page, status := resetPasswordPage(resetToken)
		c.Header("Cache-Control", "no-store")
		c.Data(status, "text/html; charset=utf-8", []byte(page))
	}
}

// HandleResetPassword godoc
// @Summary      Reset password
// @Description  Set a new password using the reset token from the forget-password mail.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        resetToken  path      string                       true  "Reset JWT from mail"
// @Param        body        body      request.ResetPasswordRequest true  "New password"
// @Success      200         {object}  map[string]string
// @Failure      400         {object}  share.ErrorResponse
// @Router       /auth/user/reset-password/{resetToken} [post]
func (ctrl AuthenticationController) HandleResetPassword() gin.HandlerFunc {
	return func(c *gin.Context) {
		resetToken := c.Param("resetToken")
		userId, err := userIDFromResetToken(resetToken)
		if err != nil {
			c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: err.Error()})
			return
		}
		var resetPasswordRequest request.ResetPasswordRequest
		if err := c.ShouldBind(&resetPasswordRequest); err != nil {
			c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: err.Error()})
			return
		}
		err = ctrl.validator.Struct(resetPasswordRequest)
		if err != nil {
			c.JSON(http.StatusBadRequest, customValidator.HandleValidationError(err))
			return
		}
		resetPasswordRepo := repository.NewCacheUserRepository(ctrl.db, ctrl.redisClient)
		resetPasswordService := service.NewResetPasswordService(resetPasswordRepo)
		msg, err := resetPasswordService.ResetPassword(c.Request.Context(), userId, resetPasswordRequest.Password, resetPasswordRequest.ConfirmPassword)
		if err != nil {
			c.JSON(http.StatusBadRequest, share.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": msg})
	}
}

func resetPasswordPage(resetToken string) (string, int) {
	claims, err := service.VerifyResetPasswordToken(resetToken)
	if err != nil {
		html, renderErr := mail.Render(mail.KindResetPasswordPage, mail.ResetPasswordPageForm{
			Valid: false,
			Error: "Token đã hết hạn hoặc không hợp lệ.",
		})
		if renderErr != nil {
			return renderErr.Error(), http.StatusInternalServerError
		}
		return html, http.StatusBadRequest
	}
	username, _ := claims["username"].(string)
	html, renderErr := mail.Render(mail.KindResetPasswordPage, mail.ResetPasswordPageForm{
		Valid:    true,
		Username: username,
	})
	if renderErr != nil {
		return renderErr.Error(), http.StatusInternalServerError
	}
	return html, http.StatusOK
}

func userIDFromResetToken(resetToken string) (uuid.UUID, error) {
	claims, err := service.VerifyResetPasswordToken(resetToken)
	if err != nil {
		return uuid.Nil, errors.New("invalid token")
	}
	idValue, ok := claims["id"].(string)
	if !ok || idValue == "" {
		return uuid.Nil, errors.New("user not found")
	}
	userId, err := uuid.Parse(idValue)
	if err != nil {
		return uuid.Nil, errors.New("user not found")
	}
	return userId, nil
}
