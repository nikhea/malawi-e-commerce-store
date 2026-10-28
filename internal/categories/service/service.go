package service

import (
	"context"
	"strings"

	"github.com/nikhea/malawi-e-commerce-store/internal/categories/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/categories/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/pagination"
	"github.com/nikhea/malawi-e-commerce-store/pkg/slug"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence.
type Repository interface {
	Create(ctx context.Context, c model.Category) (model.Category, error)
	GetByID(ctx context.Context, id string) (model.Category, error)
	GetBySlug(ctx context.Context, slug string) (model.Category, error)
	List(ctx context.Context, f model.ListFilter) ([]model.Category, int64, error)
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
	slugStr := strings.ToLower(strings.TrimSpace(in.Slug))
	if slugStr == "" {
		slugStr = slug.Make(name)
	}
	if slugStr == "" {
		return public.Category{}, apperr.Validation("slug is required")
	}
	if in.ParentID != nil {
		if _, err := s.repo.GetByID(ctx, *in.ParentID); err != nil {
			return public.Category{}, err // NOT_FOUND passes through
		}
	}

	c, err := s.repo.Create(ctx, model.Category{
		Name: name, Slug: slugStr,
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

var categorySorts = map[string]bool{
	"name_asc":           true,
	"name_desc":          true,
	"newest":             true,
	"oldest":             true,
	"product_count_asc":  true,
	"product_count_desc": true,
}

func (s *service) List(ctx context.Context, f public.ListFilter) (public.Page, error) {
	if f.Limit < 0 {
		return public.Page{}, apperr.Validation("limit cannot be negative")
	}
	if f.Offset < 0 {
		return public.Page{}, apperr.Validation("offset cannot be negative")
	}
	if f.Page < 0 {
		return public.Page{}, apperr.Validation("page cannot be negative")
	}
	if f.RootOnly && f.ParentID != nil {
		return public.Page{}, apperr.Validation("root_only cannot be combined with parent_id")
	}

	sortKey := strings.TrimSpace(f.Sort)
	if sortKey == "" {
		sortKey = "name_asc"
	}
	if !categorySorts[sortKey] {
		return public.Page{}, apperr.Validation("unknown sort; use name_asc, name_desc, newest, oldest, product_count_asc or product_count_desc")
	}

	limit, offset := pagination.Normalize(f.Page, f.Limit, f.Offset)
	parentID := f.ParentID
	if parentID != nil {
		trimmed := strings.TrimSpace(*parentID)
		if trimmed == "" {
			parentID = nil
		} else {
			parentID = &trimmed
		}
	}

	cats, total, err := s.repo.List(ctx, model.ListFilter{
		Query: strings.TrimSpace(f.Query), ParentID: parentID, RootOnly: f.RootOnly,
		HasProducts: f.HasProducts, Sort: sortKey, Limit: limit, Offset: offset,
	})
	if err != nil {
		return public.Page{}, err
	}
	out := make([]public.Category, 0, len(cats))
	for _, c := range cats {
		out = append(out, toPublic(c))
	}
	return public.Page{Items: out, Meta: pagination.NewMeta(total, limit, offset)}, nil
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
