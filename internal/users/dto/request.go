package dto

import "github.com/nikhea/malawi-e-commerce-store/internal/users/public"

// SetRoleRequest is the PATCH /admin/users/:id/role body. Exported so
// swag can render it in generated API docs.
type SetRoleRequest struct {
	Role public.Role `json:"role" binding:"required" example:"admin"`
}
