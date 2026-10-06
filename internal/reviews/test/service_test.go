package test

import (
	"context"
	"testing"

	productspublic "github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews/service"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// fakeRepo is an in-memory review store.
type fakeRepo struct {
	byID      map[string]model.Review
	byPair    map[string]string // user|product → id
	byProduct map[string][]string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byID: map[string]model.Review{}, byPair: map[string]string{}, byProduct: map[string][]string{}}
}

func (f *fakeRepo) Create(_ context.Context, r model.Review) (model.Review, error) {
	k := r.UserID + "|" + r.ProductID
	if _, taken := f.byPair[k]; taken {
		return model.Review{}, apperr.Conflict("review already exists")
	}
	r.ID = "rev-" + r.UserID
	f.byID[r.ID] = r
	f.byPair[k] = r.ID
	f.byProduct[r.ProductID] = append(f.byProduct[r.ProductID], r.ID)
	return r, nil
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (model.Review, error) {
	r, ok := f.byID[id]
	if !ok {
		return model.Review{}, apperr.NotFound("review not found")
	}
	return r, nil
}

func (f *fakeRepo) ListByProduct(_ context.Context, productID string) ([]model.Review, error) {
	out := []model.Review{}
	for _, id := range f.byProduct[productID] {
		out = append(out, f.byID[id])
	}
	return out, nil
}

func (f *fakeRepo) Stats(_ context.Context, productID string) (float64, int, error) {
	ids := f.byProduct[productID]
	if len(ids) == 0 {
		return 0, 0, nil
	}
	sum := 0
	for _, id := range ids {
		sum += f.byID[id].Rating
	}
	return float64(sum) / float64(len(ids)), len(ids), nil
}

func (f *fakeRepo) Update(_ context.Context, r model.Review) (model.Review, error) {
	if _, ok := f.byID[r.ID]; !ok {
		return model.Review{}, apperr.NotFound("review not found")
	}
	f.byID[r.ID] = r
	return r, nil
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	r, ok := f.byID[id]
	if !ok {
		return apperr.NotFound("review not found")
	}
	delete(f.byPair, r.UserID+"|"+r.ProductID)
	delete(f.byID, id)
	kept := f.byProduct[r.ProductID][:0]
	for _, other := range f.byProduct[r.ProductID] {
		if other != id {
			kept = append(kept, other)
		}
	}
	f.byProduct[r.ProductID] = kept
	return nil
}

// fakeProducts knows "prod-1" only.
type fakeProducts struct{}

func (fakeProducts) Create(context.Context, productspublic.CreateProductInput) (productspublic.Product, error) {
	return productspublic.Product{}, nil
}
func (fakeProducts) GetByID(_ context.Context, id string) (productspublic.Detail, error) {
	if id != "prod-1" {
		return productspublic.Detail{}, apperr.NotFound("product not found")
	}
	return productspublic.Detail{Product: productspublic.Product{ID: "prod-1"}}, nil
}
func (fakeProducts) GetBySlug(context.Context, string) (productspublic.Detail, error) {
	return productspublic.Detail{}, apperr.NotFound("product not found")
}
func (fakeProducts) List(context.Context, productspublic.ListFilter) ([]productspublic.Product, error) {
	return nil, nil
}
func (fakeProducts) Update(context.Context, string, productspublic.UpdateProductInput) (productspublic.Product, error) {
	return productspublic.Product{}, nil
}
func (fakeProducts) Delete(context.Context, string) error { return nil }
func (fakeProducts) SetImage(context.Context, string, string, string) (productspublic.Product, error) {
	return productspublic.Product{}, nil
}

// fakeUsers knows "u1" (Aisha) and "u2" only.
type fakeUsers struct{}

func (fakeUsers) Create(context.Context, userspublic.CreateUserInput) (userspublic.User, error) {
	return userspublic.User{}, nil
}
func (fakeUsers) GetByID(_ context.Context, id string) (userspublic.User, error) {
	if id == "u1" {
		return userspublic.User{ID: "u1", Name: "Aisha", Email: "a@malawi.mw"}, nil
	}
	if id == "u2" {
		return userspublic.User{ID: "u2", Email: "b@malawi.mw"}, nil
	}
	return userspublic.User{}, apperr.NotFound("user not found")
}
func (fakeUsers) GetByEmail(context.Context, string) (userspublic.User, error) {
	return userspublic.User{}, apperr.NotFound("user not found")
}
func (fakeUsers) GetCredentials(context.Context, string) (userspublic.Credentials, error) {
	return userspublic.Credentials{}, apperr.NotFound("user not found")
}
func (fakeUsers) SetRole(context.Context, string, userspublic.Role) (userspublic.User, error) {
	return userspublic.User{}, nil
}
func (fakeUsers) SetEmailVerified(context.Context, string, bool) (userspublic.User, error) {
	return userspublic.User{}, nil
}
func (fakeUsers) SetPasswordHash(context.Context, string, string) (userspublic.User, error) {
	return userspublic.User{}, nil
}

func newService() public.Service {
	return service.NewService(newFakeRepo(), fakeProducts{}, fakeUsers{})
}

func TestReviewLifecycle(t *testing.T) {
	ctx := context.Background()
	svc := newService()

	rev, err := svc.Create(ctx, public.CreateReviewInput{
		UserID: "u1", ProductID: "prod-1", Rating: 5, Title: "Great", Body: "Lasted long",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if rev.Author != "Aisha" {
		t.Fatalf("author snapshot wrong: %+v", rev)
	}

	// Second review by same user conflicts.
	if _, err := svc.Create(ctx, public.CreateReviewInput{
		UserID: "u1", ProductID: "prod-1", Rating: 1,
	}); apperr.CodeOf(err) != apperr.CodeConflict {
		t.Fatalf("expected CONFLICT, got %v", err)
	}

	// Rating bounds.
	if _, err := svc.Create(ctx, public.CreateReviewInput{
		UserID: "u2", ProductID: "prod-1", Rating: 6,
	}); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("expected VALIDATION_ERROR, got %v", err)
	}

	// Stranger can't touch it.
	if _, err := svc.UpdateMine(ctx, "u2", rev.ID, public.UpdateReviewInput{}); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}

	// Owner edits.
	updated, err := svc.UpdateMine(ctx, "u1", rev.ID, public.UpdateReviewInput{Rating: intptr(4)})
	if err != nil || updated.Rating != 4 {
		t.Fatalf("update: %v %+v", err, updated)
	}

	// Summary aggregates.
	if _, err := svc.Create(ctx, public.CreateReviewInput{
		UserID: "u2", ProductID: "prod-1", Rating: 2,
	}); err != nil {
		t.Fatalf("second review: %v", err)
	}
	summary, err := svc.Summary(ctx, "prod-1")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.Count != 2 || summary.Average != 3.0 || len(summary.Reviews) != 2 {
		t.Fatalf("bad summary: %+v", summary)
	}

	// Delete own; admin removes the other.
	if err := svc.DeleteMine(ctx, "u1", rev.ID); err != nil {
		t.Fatalf("delete mine: %v", err)
	}
	summary, err = svc.Summary(ctx, "prod-1")
	if err != nil || summary.Count != 1 {
		t.Fatalf("after delete: %v %+v", err, summary)
	}
	if err := svc.DeleteAny(ctx, summary.Reviews[0].ID); err != nil {
		t.Fatalf("admin delete: %v", err)
	}
	summary, err = svc.Summary(ctx, "prod-1")
	if err != nil || summary.Count != 0 || summary.Average != 0 {
		t.Fatalf("empty summary wrong: %v %+v", err, summary)
	}
	if err := svc.DeleteAny(ctx, "missing"); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}
}

func intptr(i int) *int { return &i }
