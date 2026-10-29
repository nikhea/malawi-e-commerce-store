package dto

// AddWishlistRequest is the POST /wishlist body.
type AddWishlistRequest struct {
	ProductID string `json:"product_id" binding:"required" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
}

// ListWishlistQuery binds GET /wishlist paging and catalog filters.
type ListWishlistQuery struct {
	Q          string  `form:"q"`
	CategoryID *string `form:"category_id"`
	Sort       string  `form:"sort"`
	Page       int     `form:"page"`
	Limit      int     `form:"limit"`
	Offset     int     `form:"offset"`
}
