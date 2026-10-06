package service

import (
	"context"
	"strings"

	productspublic "github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence.
type Repository interface {
	Create(ctx context.Context, rev model.Review) (model.Review, error)
	GetByID(ctx context.Context, id string) (model.Review, error)
	ListByProduct(ctx context.Context, productID string) ([]model.Review, error)
	Stats(ctx context.Context, productID string) (float64, int, error)
	Update(ctx context.Context, rev model.Review) (model.Review, error)
	Delete(ctx context.Context, id string) error
}

type service struct {
	repo     Repository
	products productspublic.Service
	users    userspublic.Service
}

func NewService(repo Repository, products productspublic.Service, users userspublic.Service) public.Service {
	return &service{repo: repo, products: products, users: users}
}

func toPublic(r model.Review) public.Review {
	return public.Review{
		ID: r.ID, UserID: r.UserID, Author: r.Author, ProductID: r.ProductID,
		Rating: r.Rating, Title: r.Title, Body: r.Body, CreatedAt: r.CreatedAt,
	}
}

func validRating(rating int) bool {
	return rating >= 1 && rating <= 5
}

func (s *service) Create(ctx context.Context, in public.CreateReviewInput) (public.Review, error) {
	if strings.TrimSpace(in.ProductID) == "" {
		return public.Review{}, apperr.Validation("product id is required")
	}
	if !validRating(in.Rating) {
		return public.Review{}, apperr.Validation("rating must be 1–5")
	}
	u, err := s.users.GetByID(ctx, in.UserID)
	if err != nil {
		return public.Review{}, err
	}
	if _, err := s.products.GetByID(ctx, in.ProductID); err != nil {
		return public.Review{}, err
	}
	author := strings.TrimSpace(u.Name)
	if author == "" {
		author = u.Email
	}

	rev, err := s.repo.Create(ctx, model.Review{
		UserID: in.UserID, Author: author, ProductID: in.ProductID,
		Rating: in.Rating,
		Title:  strings.TrimSpace(in.Title), Body: strings.TrimSpace(in.Body),
	})
	if err != nil {
		return public.Review{}, err // CONFLICT / product NOT_FOUND pass through
	}
	return toPublic(rev), nil
}

// owned fetches the review and proves the caller wrote it. Same
// don't-leak rule as orders: strangers get NOT_FOUND.
func (s *service) owned(ctx context.Context, userID, reviewID string) (model.Review, error) {
	rev, err := s.repo.GetByID(ctx, reviewID)
	if err != nil {
		return model.Review{}, err
	}
	if rev.UserID != userID {
		return model.Review{}, apperr.NotFound("review not found")
	}
	return rev, nil
}

func (s *service) UpdateMine(ctx context.Context, userID, reviewID string, in public.UpdateReviewInput) (public.Review, error) {
	rev, err := s.owned(ctx, userID, reviewID)
	if err != nil {
		return public.Review{}, err
	}
	if in.Rating != nil {
		if !validRating(*in.Rating) {
			return public.Review{}, apperr.Validation("rating must be 1–5")
		}
		rev.Rating = *in.Rating
	}
	if in.Title != nil {
		rev.Title = strings.TrimSpace(*in.Title)
	}
	if in.Body != nil {
		rev.Body = strings.TrimSpace(*in.Body)
	}
	updated, err := s.repo.Update(ctx, rev)
	if err != nil {
		return public.Review{}, err
	}
	return toPublic(updated), nil
}

func (s *service) DeleteMine(ctx context.Context, userID, reviewID string) error {
	if _, err := s.owned(ctx, userID, reviewID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, reviewID)
}

func (s *service) DeleteAny(ctx context.Context, reviewID string) error {
	if strings.TrimSpace(reviewID) == "" {
		return apperr.Validation("review id is required")
	}
	return s.repo.Delete(ctx, reviewID)
}

func (s *service) Summary(ctx context.Context, productID string) (public.Summary, error) {
	if strings.TrimSpace(productID) == "" {
		return public.Summary{}, apperr.Validation("product id is required")
	}
	avg, count, err := s.repo.Stats(ctx, productID)
	if err != nil {
		return public.Summary{}, err
	}
	reviews, err := s.repo.ListByProduct(ctx, productID)
	if err != nil {
		return public.Summary{}, err
	}
	out := make([]public.Review, 0, len(reviews))
	for _, r := range reviews {
		out = append(out, toPublic(r))
	}
	return public.Summary{Average: avg, Count: count, Reviews: out}, nil
}
