package auth

import "time"

type SigninRequest struct {
	Email    string `json:"email" validate:"required,email" example:"user@example.com"`
	Password string `json:"password" validate:"required" example:"P@ssw0rd1"`
}

type SignupRequest struct {
	Email           string `json:"email" validate:"required,email" example:"user@example.com"`
	Password        string `json:"password" validate:"required,min=8" example:"P@ssw0rd1"`
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=Password" example:"P@ssw0rd1"`
}

type AuthUserResponse struct {
	ID            string `json:"id" example:"usr_abc123"`
	Email         string `json:"email" example:"user@example.com"`
	Name          string `json:"name" example:"user"`
	EmailVerified bool   `json:"email_verified" example:"false"`
}

type AuthResponse struct {
	Success bool             `json:"success" example:"true"`
	Status  string           `json:"status" example:"success"`
	Data    AuthResponseData `json:"data"`
}

type AuthResponseData struct {
	Token            string           `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	TokenType        string           `json:"token_type" example:"Bearer"`
	ExpiresAt        time.Time        `json:"expires_at"`
	RefreshExpiresAt time.Time        `json:"refresh_expires_at"`
	User             AuthUserResponse `json:"user"`
}

type AuthErrorResponse struct {
	Success bool   `json:"success" example:"false"`
	Status  string `json:"status" example:"failure"`
	Error   string `json:"error" example:"authentication required"`
}

type LogoutResponse struct {
	Success bool               `json:"success" example:"true"`
	Status  string             `json:"status" example:"success"`
	Data    LogoutResponseData `json:"data"`
}

type LogoutResponseData struct{}

// signinDoc godoc
// @Summary Sign in
// @Description Authenticates a user with e-mail and password, sets session and refresh-token cookies, and returns a JWT bearer token.
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body SigninRequest true "Signin payload"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} AuthErrorResponse
// @Failure 401 {object} AuthErrorResponse
// @Router /signin [post]
func signinDoc() {}

// signupDoc godoc
// @Summary Sign up
// @Description Registers a user with e-mail and password, sets session and refresh-token cookies, and returns a JWT bearer token.
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body SignupRequest true "Signup payload"
// @Success 201 {object} AuthResponse
// @Failure 400 {object} AuthErrorResponse
// @Router /signup [post]
func signupDoc() {}

// refreshDoc godoc
// @Summary Refresh auth token
// @Description Rotates the refresh-token cookie and returns a new JWT bearer token.
// @Tags auth
// @Produce json
// @Success 200 {object} AuthResponse
// @Failure 401 {object} AuthErrorResponse
// @Failure 500 {object} AuthErrorResponse
// @Router /refresh [post]
func refreshDoc() {}

// logoutDoc godoc
// @Summary Log out
// @Description Clears Authboss session state, revokes the current bearer-token session when one is provided, and clears the refresh-token cookie.
// @Tags auth
// @Produce json
// @Success 200 {object} LogoutResponse
// @Failure 500 {object} AuthErrorResponse
// @Router /logout [delete]
func logoutDoc() {}
