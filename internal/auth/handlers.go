package auth

import "time"

type SigninRequest struct {
	Email    string `json:"email" validate:"required,email" example:"user@example.com"`
	Password string `json:"password" validate:"required,max=72" example:"P@ssw0rd1"`
}

type SignupRequest struct {
	Email           string `json:"email" validate:"required,email" example:"user@example.com"`
	Password        string `json:"password" validate:"required,min=8,max=72" example:"P@ssw0rd1"`
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
	Session AuthSessionResponse `json:"session"`
	User    AuthUserResponse    `json:"user"`
}

type AuthSessionResponse struct {
	ExpiresAt time.Time `json:"expires_at"`
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
// @Description Authenticates a user with e-mail and password and sets an HttpOnly database-backed session cookie.
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
// @Description Registers a user with e-mail and password and sets an HttpOnly database-backed session cookie.
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body SignupRequest true "Signup payload"
// @Success 201 {object} AuthResponse
// @Failure 400 {object} AuthErrorResponse
// @Router /signup [post]
func signupDoc() {}

// logoutDoc godoc
// @Summary Log out
// @Description Clears the current database-backed session cookie.
// @Tags auth
// @Produce json
// @Success 200 {object} LogoutResponse
// @Failure 500 {object} AuthErrorResponse
// @Router /logout [delete]
func logoutDoc() {}
