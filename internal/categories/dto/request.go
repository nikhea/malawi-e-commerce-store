package dto

// CreateCategoryRequest is the POST /admin/categories body. ImageURL is
// an optional direct link; omit it to upload a file later instead.
type CreateCategoryRequest struct {
	Name        string  `json:"name" binding:"required" example:"Cooking Oil"`
	Slug        string  `json:"slug" example:"cooking-oil"`
	Description string  `json:"description" example:"Everyday essentials"`
	ParentID    *string `json:"parent_id,omitempty"`
	ImageURL    string  `json:"image_url,omitempty" example:"https://…/oil.jpg"`
}

// UpdateCategoryRequest is the PATCH /admin/categories/:id body.
// Pointers distinguish "absent" from "empty".
type UpdateCategoryRequest struct {
	Name        *string `json:"name,omitempty" example:"Cooking Oil"`
	Description *string `json:"description,omitempty"`
	ParentID    *string `json:"parent_id,omitempty"`
	ClearParent bool    `json:"clear_parent,omitempty"`
}
