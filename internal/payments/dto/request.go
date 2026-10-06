package dto

// CreateIntentRequest is the POST /payments/intents body.
type CreateIntentRequest struct {
	OrderID string `json:"order_id" binding:"required" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
}
