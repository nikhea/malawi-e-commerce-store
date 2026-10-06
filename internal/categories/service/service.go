package service

import (
	"context"
	"strings"
	"unicode"

	"github.com/nikhea/malawi-e-commerce-store/internal/categories/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/categories/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence.
type Repository interface {
	Create(ctx context.Context, c model.Category) (model.Category, error)
	GetByID(ctx context.Context, id string) (model.Category, error)
	GetBySlug(ctx context.Context, slug string) (model.Category, error)
	List(ctx context.Context) ([]model.Category, error)
	Update(ctx context.Context, c model.Category) (model.Category, error)
	Delete(ctx context.Context, id string) error
	SetImage(ctx context.Context, id, url, publicID string) (model.Category, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) public.Service {
	return &service{repo: repo}
}

// slugify turns "Cooking Oil & More!" into "cooking-oil-more".
func slugify(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prevHyphen := true // trim leading hyphens
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		case r == ' ' || r == '_' || r == '-':
			if !prevHyphen {
				b.WriteRune('-')
				prevHyphen = true
			}
		case unicode.IsLetter(r):
			// Non-ASCII letters (e.g. Chichewa diacritics) are dropped;
			// slugs stay URL-safe ASCII.
		}
	}
	return strings.Trim(b.String(), "-")
}

func toPublic(c model.Category) public.Category {
	return public.Category{
		ID: c.ID, Name: c.Name, Slug: c.Slug, Description: c.Description,
		ParentID: c.ParentID, ImageURL: c.ImageURL, CreatedAt: c.CreatedAt,
	}
}

func (s *service) Create(ctx context.Context, in public.CreateCategoryInput) (public.Category, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return public.Category{}, apperr.Validation("name is required")
	}
	slug := strings.ToLower(strings.TrimSpace(in.Slug))
	if slug == "" {
		slug = slugify(name)
	}
	if slug == "" {
		return public.Category{}, apperr.Validation("slug is required")
	}
	if in.ParentID != nil {
		if _, err := s.repo.GetByID(ctx, *in.ParentID); err != nil {
			return public.Category{}, err // NOT_FOUND passes through
		}
	}

	c, err := s.repo.Create(ctx, model.Category{
		Name: name, Slug: slug,
		Description: strings.TrimSpace(in.Description),
		ParentID:    in.ParentID,
		ImageURL:    strings.TrimSpace(in.ImageURL),
	})
	if err != nil {
		return public.Category{}, err // slug CONFLICT passes through
	}
	return toPublic(c), nil
}

func (s *service) GetByID(ctx context.Context, id string) (public.Category, error) {
	if strings.TrimSpace(id) == "" {
		return public.Category{}, apperr.Validation("id is required")
	}
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return public.Category{}, err
	}
	return toPublic(c), nil
}

func (s *service) GetBySlug(ctx context.Context, slug string) (public.Category, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return public.Category{}, apperr.Validation("slug is required")
	}
	c, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return public.Category{}, err
	}
	return toPublic(c), nil
}

func (s *service) List(ctx context.Context) ([]public.Category, error) {
	cats, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]public.Category, 0, len(cats))
	for _, c := range cats {
		out = append(out, toPublic(c))
	}
	return out, nil
}

func (s *service) Update(ctx context.Context, id string, in public.UpdateCategoryInput) (public.Category, error) {
	if strings.TrimSpace(id) == "" {
		return public.Category{}, apperr.Validation("id is required")
	}
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return public.Category{}, err
	}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return public.Category{}, apperr.Validation("name is required")
		}
		current.Name = name
	}
	if in.Description != nil {
		current.Description = strings.TrimSpace(*in.Description)
	}
	switch {
	case in.ClearParent:
		current.ParentID = nil
	case in.ParentID != nil:
		if *in.ParentID == id {
			return public.Category{}, apperr.Validation("category cannot be its own parent")
		}
		if _, err := s.repo.GetByID(ctx, *in.ParentID); err != nil {
			return public.Category{}, err
		}
		current.ParentID = in.ParentID
	}
	// NOTE: slug is immutable (URLs must not rot). Deep cycle checks are
	// out of scope for v1; self-parenting is rejected above.

	updated, err := s.repo.Update(ctx, current)
	if err != nil {
		return public.Category{}, err
	}
	return toPublic(updated), nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return apperr.Validation("id is required")
	}
	return s.repo.Delete(ctx, id)
}

func (s *service) SetImage(ctx context.Context, id, url, publicID string) (public.Category, error) {
	if strings.TrimSpace(id) == "" {
		return public.Category{}, apperr.Validation("id is required")
	}
	if strings.TrimSpace(url) == "" {
		return public.Category{}, apperr.Validation("image url is required")
	}
	c, err := s.repo.SetImage(ctx, id, strings.TrimSpace(url), strings.TrimSpace(publicID))
	if err != nil {
		return public.Category{}, err
	}
	return toPublic(c), nil
}
