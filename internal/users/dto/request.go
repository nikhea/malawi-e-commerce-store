package dto

import "github.com/nikhea/malawi-e-commerce-store/internal/users/public"

// SetRoleRequest is the PATCH /admin/users/:id/role body. Exported so
// swag can render it in generated API docs.
type SetRoleRequest struct {
	Role public.Role `json:"role" binding:"required" example:"admin"`
}

// ListUsersQuery binds the admin user directory filters. Page wins over
// offset when both are sent.
type ListUsersQuery struct {
	Q             string `form:"q"`
	Role          string `form:"role"`
	EmailVerified *bool  `form:"email_verified"`
	CreatedAfter  string `form:"created_after"`
	CreatedBefore string `form:"created_before"`
	Sort          string `form:"sort"`
	Page          int    `form:"page"`
	Limit         int    `form:"limit"`
	Offset        int    `form:"offset"`
}
