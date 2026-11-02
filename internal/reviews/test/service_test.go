package test

import (
	"context"
	"sort"
	"testing"
	"time"

	productspublic "github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews/service"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	bulkdto "github.com/nikhea/malawi-e-commerce-store/pkg/bulk/dto"
)

// fakeRepo is an in-memory review store.
type fakeRepo struct {
	byID      map[string]model.Review
	byPair    map[string]string // user|product → id
	byProduct map[string][]string
	seq       int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byID: map[string]model.Review{}, byPair: map[string]string{}, byProduct: map[string][]string{}}
}

func (f *fakeRepo) Create(_ context.Context, r model.Review) (model.Review, error) {
	k := r.UserID + "|" + r.ProductID
	if _, taken := f.byPair[k]; taken {
		return model.Review{}, apperr.Conflict("review already exists")
	}
	f.seq++
	r.ID = "rev-" + r.UserID + "-" + r.ProductID
	r.CreatedAt = time.Unix(0, int64(f.seq))
	f.byID[r.ID] = r
	f.byPair[k] = r.ID
	f.byProduct[r.ProductID] = append(f.byProduct[r.ProductID], r.ID)
	return r, nil
}

// ListByUser mirrors the repository join: owner predicate plus the product
// columns the service attaches to each row.
func (f *fakeRepo) ListByUser(_ context.Context, userID string, filter model.ListFilter) ([]model.MyReviewRow, int64, error) {
	rows := []model.MyReviewRow{}
	for _, r := range f.byID {
		if r.UserID != userID {
			continue
		}
		rows = append(rows, model.MyReviewRow{
			Review: r, ProductName: "Oil " + r.ProductID,
			ProductSlug: "oil-" + r.ProductID, ProductImageURL: "img-" + r.ProductID,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		switch filter.Sort {
		case "oldest":
			return rows[i].CreatedAt.Before(rows[j].CreatedAt)
		case "highest":
			return rows[i].Rating > rows[j].Rating
		case "lowest":
			return rows[i].Rating < rows[j].Rating
		default: // newest
			return rows[i].CreatedAt.After(rows[j].CreatedAt)
		}
	})
	total := int64(len(rows))
	start := filter.Offset
	if start > len(rows) {
		start = len(rows)
	}
	end := len(rows)
	if filter.Limit > 0 && start+filter.Limit < end {
		end = start + filter.Limit
	}
	return rows[start:end], total, nil
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (model.Review, error) {
	r, ok := f.byID[id]
	if !ok {
		return model.Review{}, apperr.NotFound("review not found")
	}
	return r, nil
}

func (f *fakeRepo) ListByProduct(_ context.Context, productID string, filter model.ListFilter) ([]model.Review, int64, error) {
	out := []model.Review{}
	for _, id := range f.byProduct[productID] {
		r := f.byID[id]
		if filter.Rating != nil && r.Rating != *filter.Rating {
			continue
		}
		out = append(out, r)
	}
	total := int64(len(out))
	start := filter.Offset
	if start > len(out) {
		start = len(out)
	}
	end := len(out)
	if filter.Limit > 0 && start+filter.Limit < end {
		end = start + filter.Limit
	}
	return out[start:end], total, nil
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

// fakeProducts knows "prod-1" and "prod-2" only.
type fakeProducts struct{}

func (fakeProducts) Create(context.Context, productspublic.CreateProductInput) (productspublic.Product, error) {
	return productspublic.Product{}, nil
}
func (fakeProducts) GetByID(_ context.Context, id string) (productspublic.Detail, error) {
	if id != "prod-1" && id != "prod-2" {
		return productspublic.Detail{}, apperr.NotFound("product not found")
	}
	return productspublic.Detail{Product: productspublic.Product{
		ID: id, Name: "Oil " + id, Slug: "oil-" + id, IsActive: true,
	}}, nil
}
func (fakeProducts) GetBySlug(context.Context, string) (productspublic.Detail, error) {
	return productspublic.Detail{}, apperr.NotFound("product not found")
}
func (fakeProducts) List(context.Context, productspublic.ListFilter) (productspublic.Page, error) {
	return productspublic.Page{}, nil
}
func (fakeProducts) Update(context.Context, string, productspublic.UpdateProductInput) (productspublic.Product, error) {
	return productspublic.Product{}, nil
}
func (fakeProducts) Delete(context.Context, string) error { return nil }
func (fakeProducts) SetImage(context.Context, string, string, string) (productspublic.Product, error) {
	return productspublic.Product{}, nil
}
func (fakeProducts) AddImage(context.Context, string) (productspublic.Image, error) {
	return productspublic.Image{}, nil
}
func (fakeProducts) ListImages(context.Context, string) ([]productspublic.Image, error) {
	return nil, nil
}
func (fakeProducts) DeleteImage(context.Context, string, string) error { return nil }
func (fakeProducts) SetPrimaryImage(context.Context, string, string) (productspublic.Product, error) {
	return productspublic.Product{}, nil
}
func (fakeProducts) CompleteImage(context.Context, string, string, string) error { return nil }

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
func (fakeUsers) List(context.Context, userspublic.ListFilter) (userspublic.Page, error) {
	return userspublic.Page{}, nil
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
	summary, err := svc.Summary(ctx, "prod-1", public.ListFilter{})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.Count != 2 || summary.Average != 3.0 || len(summary.Reviews) != 2 {
		t.Fatalf("bad summary: %+v", summary)
	}
	if summary.Meta.Limit != 12 || summary.Meta.Total != 2 {
		t.Fatalf("bad summary meta: %+v", summary.Meta)
	}

	// Filters narrow the page but not the aggregate.
	rating := 2
	filtered, err := svc.Summary(ctx, "prod-1", public.ListFilter{Rating: &rating, Limit: 1})
	if err != nil {
		t.Fatalf("filtered summary: %v", err)
	}
	if filtered.Count != 2 || filtered.Meta.Total != 1 || len(filtered.Reviews) != 1 || filtered.Reviews[0].Rating != 2 {
		t.Fatalf("rating filter wrong: %+v", filtered)
	}
	if _, err := svc.Summary(ctx, "prod-1", public.ListFilter{Rating: intptr(9)}); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("bad rating: expected VALIDATION_ERROR, got %v", err)
	}
	if _, err := svc.Summary(ctx, "prod-1", public.ListFilter{Sort: "random"}); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("bad sort: expected VALIDATION_ERROR, got %v", err)
	}

	// Delete own; admin removes the other.
	if err := svc.DeleteMine(ctx, "u1", rev.ID); err != nil {
		t.Fatalf("delete mine: %v", err)
	}
	summary, err = svc.Summary(ctx, "prod-1", public.ListFilter{})
	if err != nil || summary.Count != 1 {
		t.Fatalf("after delete: %v %+v", err, summary)
	}
	if err := svc.DeleteAny(ctx, summary.Reviews[0].ID); err != nil {
		t.Fatalf("admin delete: %v", err)
	}
	summary, err = svc.Summary(ctx, "prod-1", public.ListFilter{})
	if err != nil || summary.Count != 0 || summary.Average != 0 {
		t.Fatalf("empty summary wrong: %v %+v", err, summary)
	}
	if err := svc.DeleteAny(ctx, "missing"); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}
}

func intptr(i int) *int { return &i }

func TestListMine(t *testing.T) {
	ctx := context.Background()
	svc := service.NewService(newFakeRepo(), fakeProducts{}, fakeUsers{})

	for _, in := range []public.CreateReviewInput{
		{UserID: "u1", ProductID: "prod-1", Rating: 5, Title: "First"},
		{UserID: "u1", ProductID: "prod-2", Rating: 3, Title: "Second"},
		{UserID: "u2", ProductID: "prod-1", Rating: 1, Title: "Other"},
	} {
		if _, err := svc.Create(ctx, in); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// Only the caller's rows, newest first, product context attached.
	page, err := svc.ListMine(ctx, "u1", public.ListFilter{})
	if err != nil {
		t.Fatalf("list mine: %v", err)
	}
	if page.Meta.Total != 2 || page.Meta.Limit != 12 || len(page.Items) != 2 {
		t.Fatalf("bad page: %+v", page)
	}
	if page.Items[0].Title != "Second" {
		t.Fatalf("expected newest first, got %+v", page.Items)
	}
	if page.Items[0].Product.ID != "prod-2" || page.Items[0].Product.Name == "" ||
		page.Items[0].Product.Slug != "oil-prod-2" || page.Items[0].Product.ImageURL != "img-prod-2" {
		t.Fatalf("product context missing: %+v", page.Items[0])
	}
	if page.Items[0].UserID != "u1" {
		t.Fatalf("leaked another user's review: %+v", page.Items[0])
	}

	// Pagination and explicit sort.
	paged, err := svc.ListMine(ctx, "u1", public.ListFilter{Sort: "oldest", Limit: 1})
	if err != nil {
		t.Fatalf("paged: %v", err)
	}
	if paged.Meta.Total != 2 || len(paged.Items) != 1 || paged.Items[0].Title != "First" {
		t.Fatalf("bad paged result: %+v", paged)
	}

	// Guardrails.
	if _, err := svc.ListMine(ctx, "", public.ListFilter{}); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("expected VALIDATION_ERROR, got %v", err)
	}
	if _, err := svc.ListMine(ctx, "u1", public.ListFilter{Sort: "random"}); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("expected VALIDATION_ERROR, got %v", err)
	}
	if _, err := svc.ListMine(ctx, "u1", public.ListFilter{Limit: -1}); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("expected VALIDATION_ERROR, got %v", err)
	}
}

// Bulk admin writes are not exercised by this module's tests.
func (fakeProducts) BulkCreate(context.Context, []productspublic.CreateProductInput, bool, bool) ([]bulkdto.BulkResult[productspublic.Product], error) {
	return nil, nil
}
func (fakeProducts) BulkUpdate(context.Context, []productspublic.BulkUpdateItem, bool, bool) ([]bulkdto.BulkResult[productspublic.Product], error) {
	return nil, nil
}
func (fakeProducts) BulkDelete(context.Context, []string, bool, bool) ([]bulkdto.BulkResult[bulkdto.Empty], error) {
	return nil, nil
}
