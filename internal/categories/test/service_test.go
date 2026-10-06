package test

import (
	"context"
	"testing"

	"github.com/nikhea/malawi-e-commerce-store/internal/categories/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/categories/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/categories/service"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// fakeRepo is an in-memory Repository. No real DB.
type fakeRepo struct {
	byID   map[string]model.Category
	bySlug map[string]string
}

func newFakeRepo(seed ...model.Category) *fakeRepo {
	f := &fakeRepo{byID: map[string]model.Category{}, bySlug: map[string]string{}}
	for _, c := range seed {
		f.byID[c.ID] = c
		f.bySlug[c.Slug] = c.ID
	}
	return f
}

func (f *fakeRepo) Create(_ context.Context, c model.Category) (model.Category, error) {
	if _, taken := f.bySlug[c.Slug]; taken {
		return model.Category{}, apperr.Conflict("slug already in use")
	}
	c.ID = "cat-" + c.Slug
	f.byID[c.ID] = c
	f.bySlug[c.Slug] = c.ID
	return c, nil
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (model.Category, error) {
	c, ok := f.byID[id]
	if !ok {
		return model.Category{}, apperr.NotFound("category not found")
	}
	return c, nil
}

func (f *fakeRepo) GetBySlug(_ context.Context, slug string) (model.Category, error) {
	id, ok := f.bySlug[slug]
	if !ok {
		return model.Category{}, apperr.NotFound("category not found")
	}
	return f.byID[id], nil
}

func (f *fakeRepo) List(_ context.Context) ([]model.Category, error) {
	out := []model.Category{}
	for _, c := range f.byID {
		out = append(out, c)
	}
	return out, nil
}

func (f *fakeRepo) Update(_ context.Context, c model.Category) (model.Category, error) {
	if _, ok := f.byID[c.ID]; !ok {
		return model.Category{}, apperr.NotFound("category not found")
	}
	f.byID[c.ID] = c
	return c, nil
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	if _, ok := f.byID[id]; !ok {
		return apperr.NotFound("category not found")
	}
	delete(f.bySlug, f.byID[id].Slug)
	delete(f.byID, id)
	return nil
}

func (f *fakeRepo) SetImage(_ context.Context, id, url, publicID string) (model.Category, error) {
	c, ok := f.byID[id]
	if !ok {
		return model.Category{}, apperr.NotFound("category not found")
	}
	c.ImageURL = url
	f.byID[id] = c
	return c, nil
}

func strptr(s string) *string { return &s }

func TestSlugify(t *testing.T) {
	tests := []struct{ name, slug string }{
		{"Cooking Oil", "cooking-oil"},
		{"  Maize   Flour  ", "maize-flour"},
		{"Fish & Chips!", "fish-chips"},
		{"Sugar_500g", "sugar-500g"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := service.NewService(newFakeRepo())
			got, err := svc.Create(context.Background(), public.CreateCategoryInput{Name: tt.name})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if got.Slug != tt.slug {
				t.Fatalf("expected slug %q, got %q", tt.slug, got.Slug)
			}
		})
	}
}

func TestCreateValidation(t *testing.T) {
	ctx := context.Background()
	svc := service.NewService(newFakeRepo())

	if _, err := svc.Create(ctx, public.CreateCategoryInput{}); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("empty name: expected VALIDATION_ERROR, got %v", err)
	}
	if _, err := svc.Create(ctx, public.CreateCategoryInput{Name: "X", ParentID: strptr("missing")}); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("bad parent: expected NOT_FOUND, got %v", err)
	}

	parent, err := svc.Create(ctx, public.CreateCategoryInput{Name: "Parent"})
	if err != nil {
		t.Fatalf("parent: %v", err)
	}
	child, err := svc.Create(ctx, public.CreateCategoryInput{Name: "Child", ParentID: &parent.ID})
	if err != nil {
		t.Fatalf("child: %v", err)
	}
	if child.ParentID == nil || *child.ParentID != parent.ID {
		t.Fatalf("parent link missing: %+v", child)
	}

	dupSvc := service.NewService(newFakeRepo())
	if _, err := dupSvc.Create(ctx, public.CreateCategoryInput{Name: "Cooking Oil"}); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := dupSvc.Create(ctx, public.CreateCategoryInput{Name: "cooking oil"}); apperr.CodeOf(err) != apperr.CodeConflict {
		t.Fatalf("duplicate slug: expected CONFLICT, got %v", err)
	}
}

func TestUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	svc := service.NewService(newFakeRepo())

	cat, err := svc.Create(ctx, public.CreateCategoryInput{Name: "Temp", Description: "d"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Self-parent rejected.
	if _, err := svc.Update(ctx, cat.ID, public.UpdateCategoryInput{ParentID: &cat.ID}); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("self parent: expected VALIDATION_ERROR, got %v", err)
	}

	// Rename keeps slug immutable.
	renamed, err := svc.Update(ctx, cat.ID, public.UpdateCategoryInput{Name: strptr("Renamed")})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if renamed.Slug != cat.Slug || renamed.Name != "Renamed" {
		t.Fatalf("bad rename: %+v", renamed)
	}

	// Detach to root.
	root, err := svc.Create(ctx, public.CreateCategoryInput{Name: "Root"})
	if err != nil {
		t.Fatalf("root: %v", err)
	}
	linked, err := svc.Update(ctx, cat.ID, public.UpdateCategoryInput{ParentID: &root.ID})
	if err != nil || linked.ParentID == nil {
		t.Fatalf("link: %v %+v", err, linked)
	}
	detached, err := svc.Update(ctx, cat.ID, public.UpdateCategoryInput{ClearParent: true})
	if err != nil || detached.ParentID != nil {
		t.Fatalf("detach: %v %+v", err, detached)
	}

	if err := svc.Delete(ctx, cat.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := svc.Delete(ctx, cat.ID); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("double delete: expected NOT_FOUND, got %v", err)
	}
}

func TestSetImage(t *testing.T) {
	ctx := context.Background()
	svc := service.NewService(newFakeRepo())

	cat, err := svc.Create(ctx, public.CreateCategoryInput{Name: "No Image"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if cat.ImageURL != "" {
		t.Fatalf("expected empty image, got %q", cat.ImageURL)
	}

	withImage, err := svc.SetImage(ctx, cat.ID, "https://cdn/x.jpg", "x")
	if err != nil {
		t.Fatalf("set image: %v", err)
	}
	if withImage.ImageURL != "https://cdn/x.jpg" {
		t.Fatalf("bad image url: %+v", withImage)
	}
	if _, err := svc.SetImage(ctx, cat.ID, "", ""); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("empty url: expected VALIDATION_ERROR, got %v", err)
	}
	if _, err := svc.SetImage(ctx, "missing", "https://cdn/x.jpg", "x"); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("unknown id: expected NOT_FOUND, got %v", err)
	}
}
