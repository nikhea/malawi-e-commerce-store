package dto

// AddWishlistRequest is the POST /wishlist body.
type AddWishlistRequest struct {
	ProductID string `json:"product_id" binding:"required" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
}
