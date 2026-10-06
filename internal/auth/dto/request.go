package dto

// RegisterRequest is the POST /auth/register body. Exported so swag can
// render it in generated API docs.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required" example:"shop@malawi.mw"`
	Name     string `json:"name" example:"Aisha Banda"`
	Password string `json:"password" binding:"required" example:"hunter2-secure"`
}

// LoginRequest is the POST /auth/login body. Exported for swag.
type LoginRequest struct {
	Email    string `json:"email" binding:"required" example:"shop@malawi.mw"`
	Password string `json:"password" binding:"required" example:"hunter2-secure"`
}
