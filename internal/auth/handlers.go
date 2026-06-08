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
	Status    string           `json:"status" example:"success"`
	Token     string           `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	TokenType string           `json:"token_type" example:"Bearer"`
	ExpiresAt time.Time        `json:"expires_at"`
	User      AuthUserResponse `json:"user"`
}

type AuthErrorResponse struct {
	Status string `json:"status" example:"failure"`
	Error  string `json:"error" example:"authentication required"`
}

// signinDoc godoc
// @Summary Sign in
// @Description Authenticates a user with e-mail and password, sets a session cookie, and returns a JWT bearer token.
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
// @Description Registers a user with e-mail and password, sets a session cookie, and returns a JWT bearer token.
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body SignupRequest true "Signup payload"
// @Success 201 {object} AuthResponse
// @Failure 400 {object} AuthErrorResponse
// @Router /signup [post]
func signupDoc() {}
