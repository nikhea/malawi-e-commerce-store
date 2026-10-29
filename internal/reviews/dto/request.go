package dto

// CreateReviewRequest is the POST /products/:id/reviews body.
type CreateReviewRequest struct {
	Rating int    `json:"rating" binding:"required" example:"5"`
	Title  string `json:"title" example:"Great oil"`
	Body   string `json:"body" example:"Lasted the whole month"`
}

// UpdateReviewRequest is the PATCH /reviews/:id body.
// Pointers distinguish "absent" from "zero".
type UpdateReviewRequest struct {
	Rating *int    `json:"rating,omitempty"`
	Title  *string `json:"title,omitempty"`
	Body   *string `json:"body,omitempty"`
}

// ListReviewsQuery binds the public product review page.
type ListReviewsQuery struct {
	Q      string `form:"q"`
	Rating *int   `form:"rating"`
	Sort   string `form:"sort"`
	Page   int    `form:"page"`
	Limit  int    `form:"limit"`
	Offset int    `form:"offset"`
}
