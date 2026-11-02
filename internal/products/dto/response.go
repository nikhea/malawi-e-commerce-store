package dto

// UploadImagesResponse reports one admin multi-file upload. Each accepted
// file becomes its own River job, so the gallery fills in asynchronously.
type UploadImagesResponse struct {
	Status string `json:"status" example:"queued"`
	Queued int    `json:"queued" example:"3"`
	Failed int    `json:"failed" example:"0"`
}
