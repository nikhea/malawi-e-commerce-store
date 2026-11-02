package service

import (
	"context"
	"fmt"
	"strings"

	categoriespublic "github.com/nikhea/malawi-e-commerce-store/internal/categories/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	variantspublic "github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/bulk"
	bulkdto "github.com/nikhea/malawi-e-commerce-store/pkg/bulk/dto"
	"github.com/nikhea/malawi-e-commerce-store/pkg/pagination"
	"github.com/nikhea/malawi-e-commerce-store/pkg/slug"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence.
type Repository interface {
	Create(ctx context.Context, p model.Product) (model.Product, error)
	// Bulk writes share one transaction. failIndex is -1 on success; when
	// >= 0 the batch rolled back and err is that item's failure.
	CreateMany(ctx context.Context, items []model.Product) ([]model.Product, int, error)
	GetByID(ctx context.Context, id string) (model.Product, error)
	GetBySlug(ctx context.Context, slug string) (model.Product, error)
	List(ctx context.Context, f model.ListFilter) ([]model.Product, int64, error)
	Update(ctx context.Context, p model.Product) (model.Product, error)
	UpdateMany(ctx context.Context, items []model.Product) ([]model.Product, int, error)
	Delete(ctx context.Context, id string) error
	DeleteMany(ctx context.Context, ids []string) (int, error)
	// Blockers counts rows that would cascade or block a delete.
	Blockers(ctx context.Context, ids []string) (map[string]model.Blockers, error)
	SetImage(ctx context.Context, id, url, publicID string) (model.Product, error)

	CreateImage(ctx context.Context, productID string) (model.Image, error)
	ListImages(ctx context.Context, productID string) ([]model.Image, error)
	GetImage(ctx context.Context, productID, imageID string) (model.Image, error)
	DeleteImage(ctx context.Context, productID, imageID string) error
	SetPrimaryImage(ctx context.Context, productID, imageID string) (model.Product, error)
	// CompleteImage returns a zero ProductID when the row no longer exists
	// (admin deleted it while the upload was queued).
	CompleteImage(ctx context.Context, imageID, url, publicID string) (model.Image, error)
	// PromoteIfEmpty makes the lowest-position ready image the primary
	// when the product has none yet. Reports whether it changed anything.
	PromoteIfEmpty(ctx context.Context, productID string) (bool, error)
}

type service struct {
	repo       Repository
	categories categoriespublic.Service
	variants   variantspublic.Service
}

func NewService(repo Repository, categories categoriespublic.Service, variants variantspublic.Service) public.Service {
	return &service{repo: repo, categories: categories, variants: variants}
}

func toPublic(p model.Product) public.Product {
	return public.Product{
		ID: p.ID, CategoryID: p.CategoryID, CategoryName: p.CategoryName,
		Name: p.Name, Slug: p.Slug, Description: p.Description,
		PriceCents: p.PriceCents, Currency: p.Currency,
		ImageURL: p.ImageURL, IsActive: p.IsActive, CreatedAt: p.CreatedAt,
	}
}

func toDetail(p model.Product, variants []variantspublic.Variant, images []model.Image) public.Detail {
	views := make([]public.VariantView, 0, len(variants))
	for _, v := range variants {
		views = append(views, public.VariantView{
			ID: v.ID, Name: v.Name, SKU: v.SKU,
			PriceCents: v.PriceCents, IsActive: v.IsActive,
			Available: v.Available, InStock: v.InStock,
		})
	}
	gallery := make([]public.Image, 0, len(images))
	for _, img := range images {
		if img.URL == "" {
			continue // still queued in the media pipeline
		}
		gallery = append(gallery, toPublicImage(img, p))
	}
	return public.Detail{Product: toPublic(p), Variants: views, Images: gallery}
}

// toPublicImage marks the row whose URL/public id matches the product's
// legacy primary image. Public-id comparison wins; URL is the fallback for
// images uploaded before the gallery existed.
func toPublicImage(img model.Image, p model.Product) public.Image {
	primary := false
	if p.ImagePublicID != "" {
		primary = img.PublicID == p.ImagePublicID
	} else {
		primary = img.URL != "" && img.URL == p.ImageURL
	}
	return public.Image{
		ID: img.ID, URL: img.URL, Position: img.Position,
		IsPrimary: primary, CreatedAt: img.CreatedAt,
	}
}

func (s *service) detail(ctx context.Context, p model.Product) (public.Detail, error) {
	variants, err := s.variants.ListByProduct(ctx, p.ID)
	if err != nil {
		return public.Detail{}, err
	}
	images, err := s.repo.ListImages(ctx, p.ID)
	if err != nil {
		return public.Detail{}, err
	}
	return toDetail(p, variants, images), nil
}

// prepareCreate validates and normalizes one new SPU, including the
// category lookup. Shared by the single and bulk create paths so the two
// can never drift.
func (s *service) prepareCreate(ctx context.Context, in public.CreateProductInput) (model.Product, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return model.Product{}, apperr.Validation("name is required")
	}
	slugStr := strings.ToLower(strings.TrimSpace(in.Slug))
	if slugStr == "" {
		slugStr = slug.Make(name)
	}
	if slugStr == "" {
		return model.Product{}, apperr.Validation("slug is required")
	}
	if in.PriceCents < 0 {
		return model.Product{}, apperr.Validation("price cannot be negative")
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = "USD"
	}
	if in.CategoryID != nil {
		if _, err := s.categories.GetByID(ctx, *in.CategoryID); err != nil {
			return model.Product{}, err // NOT_FOUND passes through
		}
	}
	return model.Product{
		CategoryID: in.CategoryID, Name: name, Slug: slugStr,
		Description: strings.TrimSpace(in.Description),
		PriceCents:  in.PriceCents, Currency: currency,
		ImageURL: strings.TrimSpace(in.ImageURL),
	}, nil
}

func (s *service) Create(ctx context.Context, in public.CreateProductInput) (public.Product, error) {
	prepared, err := s.prepareCreate(ctx, in)
	if err != nil {
		return public.Product{}, err
	}
	p, err := s.repo.Create(ctx, prepared)
	if err != nil {
		return public.Product{}, err
	}
	return toPublic(p), nil
}

func (s *service) GetByID(ctx context.Context, id string) (public.Detail, error) {
	if strings.TrimSpace(id) == "" {
		return public.Detail{}, apperr.Validation("id is required")
	}
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return public.Detail{}, err
	}
	return s.detail(ctx, p)
}

func (s *service) GetBySlug(ctx context.Context, slugStr string) (public.Detail, error) {
	slugStr = strings.ToLower(strings.TrimSpace(slugStr))
	if slugStr == "" {
		return public.Detail{}, apperr.Validation("slug is required")
	}
	p, err := s.repo.GetBySlug(ctx, slugStr)
	if err != nil {
		return public.Detail{}, err
	}
	return s.detail(ctx, p)
}

var productSorts = map[string]bool{
	"name_asc":    true,
	"name_desc":   true,
	"price_asc":   true,
	"price_desc":  true,
	"newest":      true,
	"oldest":      true,
	"rating_asc":  true,
	"rating_desc": true,
	"relevance":   true,
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
	if f.MinPriceCents != nil && *f.MinPriceCents < 0 {
		return public.Page{}, apperr.Validation("min_price_cents cannot be negative")
	}
	if f.MaxPriceCents != nil && *f.MaxPriceCents < 0 {
		return public.Page{}, apperr.Validation("max_price_cents cannot be negative")
	}
	if f.MinPriceCents != nil && f.MaxPriceCents != nil && *f.MinPriceCents > *f.MaxPriceCents {
		return public.Page{}, apperr.Validation("min_price_cents cannot exceed max_price_cents")
	}
	if f.MinRating != nil {
		if *f.MinRating < 0 || *f.MinRating > 5 {
			return public.Page{}, apperr.Validation("min_rating must be between 0 and 5")
		}
		if *f.MinRating == 0 {
			f.MinRating = nil // 0 means "no rating floor", not "unreviewed only"
		}
	}
	if f.CreatedAfter != nil && f.CreatedBefore != nil && f.CreatedAfter.After(*f.CreatedBefore) {
		return public.Page{}, apperr.Validation("created_after cannot be after created_before")
	}

	query := strings.TrimSpace(f.Query)
	sortKey := strings.TrimSpace(f.Sort)
	if sortKey == "" {
		// Search results read best by relevance; browsing keeps the
		// historical name order.
		if query != "" {
			sortKey = "relevance"
		} else {
			sortKey = "name_asc"
		}
	}
	if !productSorts[sortKey] {
		return public.Page{}, apperr.Validation("unknown sort; use name_asc, name_desc, price_asc, price_desc, newest, oldest, rating_asc, rating_desc or relevance")
	}
	if sortKey == "relevance" && query == "" {
		return public.Page{}, apperr.Validation("relevance sort requires q")
	}

	limit, offset := pagination.NormalizeWithDefault(f.Page, f.Limit, f.Offset, pagination.DefaultListLimit)
	size := strings.TrimSpace(f.Size)
	categoryID := trimOptional(f.CategoryID)
	categorySlug := trimOptional(f.CategorySlug)
	ids := make([]string, 0, len(f.IDs))
	for _, id := range f.IDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			ids = append(ids, trimmed)
		}
	}

	products, total, err := s.repo.List(ctx, model.ListFilter{
		IDs: ids, Query: query, CategoryID: categoryID, CategorySlug: categorySlug,
		MinPriceCents: f.MinPriceCents, MaxPriceCents: f.MaxPriceCents,
		InStock: f.InStock, Size: size, MinRating: f.MinRating,
		CreatedAfter: f.CreatedAfter, CreatedBefore: f.CreatedBefore,
		ActiveOnly: f.ActiveOnly, Sort: sortKey, Limit: limit, Offset: offset,
	})
	if err != nil {
		return public.Page{}, err
	}
	out := make([]public.Product, 0, len(products))
	for _, p := range products {
		out = append(out, toPublic(p))
	}
	return public.Page{Items: out, Meta: pagination.NewMeta(total, limit, offset)}, nil
}

// trimOptional turns blank optional filters into nil so the repository
// does not emit predicates for empty query-string values.
func trimOptional(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// applyUpdate projects a patch onto the current row. It performs no writes:
// single update, bulk update and dry runs share this exact semantics.
func (s *service) applyUpdate(ctx context.Context, current model.Product, in public.UpdateProductInput) (model.Product, error) {
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return model.Product{}, apperr.Validation("name is required")
		}
		current.Name = name
	}
	if in.Description != nil {
		current.Description = strings.TrimSpace(*in.Description)
	}
	if in.PriceCents != nil {
		if *in.PriceCents < 0 {
			return model.Product{}, apperr.Validation("price cannot be negative")
		}
		current.PriceCents = *in.PriceCents
	}
	if in.Currency != nil {
		currency := strings.ToUpper(strings.TrimSpace(*in.Currency))
		if currency == "" {
			return model.Product{}, apperr.Validation("currency is required")
		}
		current.Currency = currency
	}
	if in.IsActive != nil {
		current.IsActive = *in.IsActive
	}
	switch {
	case in.ClearCategory:
		current.CategoryID = nil
	case in.CategoryID != nil:
		if _, err := s.categories.GetByID(ctx, *in.CategoryID); err != nil {
			return model.Product{}, err
		}
		current.CategoryID = in.CategoryID
	}
	// NOTE: slug is immutable (URLs must not rot).
	return current, nil
}

func (s *service) Update(ctx context.Context, id string, in public.UpdateProductInput) (public.Product, error) {
	if strings.TrimSpace(id) == "" {
		return public.Product{}, apperr.Validation("id is required")
	}
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return public.Product{}, err
	}
	projected, err := s.applyUpdate(ctx, current, in)
	if err != nil {
		return public.Product{}, err
	}
	updated, err := s.repo.Update(ctx, projected)
	if err != nil {
		return public.Product{}, err
	}
	return toPublic(updated), nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return apperr.Validation("id is required")
	}
	return s.repo.Delete(ctx, id)
}

// BulkCreate creates many products. Validation is per item; a duplicate
// resulting slug is a whole-request rejection because it makes the batch
// ambiguous. With atomic=true nothing is written unless every item is valid.
func (s *service) BulkCreate(ctx context.Context, items []public.CreateProductInput, atomic, dryRun bool) ([]bulkdto.BulkResult[public.Product], error) {
	if err := bulk.CheckCount(len(items), bulk.MaxItems()); err != nil {
		return nil, err
	}

	prepared := make([]model.Product, len(items))
	prepErrs := make([]error, len(items))
	slugs := make([]string, 0, len(items))
	for i, in := range items {
		p, err := s.prepareCreate(ctx, in)
		prepared[i], prepErrs[i] = p, err
		if err == nil {
			slugs = append(slugs, p.Slug)
		}
	}
	if dup, ok := bulk.FindDuplicate(slugs); ok {
		return nil, apperr.Duplicate("duplicate product slug: " + dup)
	}

	results := make([]bulkdto.BulkResult[public.Product], len(items))
	for i := range items {
		results[i] = bulkdto.BulkResult[public.Product]{Index: i, ID: prepared[i].Slug, Status: "ok"}
		if prepErrs[i] != nil {
			results[i] = bulk.Failed[public.Product](i, prepared[i].Slug, prepErrs[i])
		}
	}

	if dryRun {
		for i := range items {
			if prepErrs[i] == nil {
				results[i] = bulk.OK(i, prepared[i].Slug, toPublic(prepared[i]))
			}
		}
		return results, nil
	}

	if atomic {
		for i, err := range prepErrs {
			if err != nil {
				return bulk.Rollback(results, i, err), nil
			}
		}
		created, failIndex, err := s.repo.CreateMany(ctx, prepared)
		if err != nil {
			if failIndex < 0 {
				failIndex = 0 // fatal tx error: surface it on the first item
			}
			return bulk.Rollback(results, failIndex, err), nil
		}
		for i := range items {
			results[i] = bulk.OK(i, created[i].Slug, toPublic(created[i]))
		}
		return results, nil
	}

	for i := range items {
		if prepErrs[i] != nil {
			continue
		}
		p, err := s.repo.Create(ctx, prepared[i])
		if err != nil {
			results[i] = bulk.Failed[public.Product](i, prepared[i].Slug, err)
			continue
		}
		results[i] = bulk.OK(i, p.Slug, toPublic(p))
	}
	return results, nil
}

func isEmptyPatch(in public.UpdateProductInput) bool {
	return in.CategoryID == nil && in.Name == nil && in.Description == nil &&
		in.PriceCents == nil && in.Currency == nil && in.IsActive == nil &&
		!in.ClearCategory
}

// BulkUpdate applies one patch per product. Empty patches are per-item
// validation errors; duplicate ids reject the whole batch.
func (s *service) BulkUpdate(ctx context.Context, items []public.BulkUpdateItem, atomic, dryRun bool) ([]bulkdto.BulkResult[public.Product], error) {
	if err := bulk.CheckCount(len(items), bulk.MaxItems()); err != nil {
		return nil, err
	}
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = strings.TrimSpace(it.ID)
	}
	if dup, ok := bulk.FindDuplicate(ids); ok {
		return nil, apperr.Duplicate("duplicate product id: " + dup)
	}

	projected := make([]model.Product, len(items))
	itemErrs := make([]error, len(items))
	results := make([]bulkdto.BulkResult[public.Product], len(items))
	for i, it := range items {
		results[i] = bulkdto.BulkResult[public.Product]{Index: i, ID: ids[i], Status: "ok"}
		if ids[i] == "" {
			itemErrs[i] = apperr.Validation("id is required")
			results[i] = bulk.Failed[public.Product](i, ids[i], itemErrs[i])
			continue
		}
		if isEmptyPatch(it.Patch) {
			itemErrs[i] = apperr.Validation("at least one field is required")
			results[i] = bulk.Failed[public.Product](i, ids[i], itemErrs[i])
			continue
		}
		current, err := s.repo.GetByID(ctx, ids[i])
		if err != nil {
			itemErrs[i] = err
			results[i] = bulk.Failed[public.Product](i, ids[i], err)
			continue
		}
		updated, err := s.applyUpdate(ctx, current, it.Patch)
		if err != nil {
			itemErrs[i] = err
			results[i] = bulk.Failed[public.Product](i, ids[i], err)
			continue
		}
		projected[i] = updated
	}

	if dryRun {
		for i := range items {
			if itemErrs[i] == nil {
				results[i] = bulk.OK(i, ids[i], toPublic(projected[i]))
			}
		}
		return results, nil
	}

	if atomic {
		for i, err := range itemErrs {
			if err != nil {
				return bulk.Rollback(results, i, err), nil
			}
		}
		rows, failIndex, err := s.repo.UpdateMany(ctx, projected)
		if err != nil {
			if failIndex < 0 {
				failIndex = 0
			}
			return bulk.Rollback(results, failIndex, err), nil
		}
		for i := range items {
			results[i] = bulk.OK(i, ids[i], toPublic(rows[i]))
		}
		return results, nil
	}

	for i := range items {
		if itemErrs[i] != nil {
			continue
		}
		row, err := s.repo.Update(ctx, projected[i])
		if err != nil {
			results[i] = bulk.Failed[public.Product](i, ids[i], err)
			continue
		}
		results[i] = bulk.OK(i, ids[i], toPublic(row))
	}
	return results, nil
}

// BulkDelete hard-deletes products, capped lower than other batches (25).
// A product referenced by order history is blocked per item: orders keep
// snapshots but must stay attributable to a real catalog row.
func (s *service) BulkDelete(ctx context.Context, ids []string, atomic, dryRun bool) ([]bulkdto.BulkResult[bulkdto.Empty], error) {
	limit := 25
	if max := bulk.MaxItems(); max < limit {
		limit = max
	}
	if err := bulk.CheckCount(len(ids), limit); err != nil {
		return nil, err
	}
	for i := range ids {
		ids[i] = strings.TrimSpace(ids[i])
	}
	if dup, ok := bulk.FindDuplicate(ids); ok {
		return nil, apperr.Duplicate("duplicate product id: " + dup)
	}
	queryIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" {
			queryIDs = append(queryIDs, id)
		}
	}
	blockers, err := s.repo.Blockers(ctx, queryIDs)
	if err != nil {
		return nil, err
	}

	results := make([]bulkdto.BulkResult[bulkdto.Empty], len(ids))
	for i, id := range ids {
		results[i] = bulkdto.BulkResult[bulkdto.Empty]{Index: i, ID: id, Status: "ok"}
		if id == "" {
			results[i] = bulk.Failed[bulkdto.Empty](i, id, apperr.Validation("id is required"))
			continue
		}
		if _, ok := blockers[id]; !ok {
			results[i] = bulk.Failed[bulkdto.Empty](i, id, apperr.NotFound("product not found"))
		}
	}
	if dryRun {
		for i, id := range ids {
			b, ok := blockers[id]
			if !ok {
				continue
			}
			results[i].WouldDelete = b.OrderItems == 0
			results[i].Blockers = blockMessages(b)
		}
		return results, nil
	}

	blocked := func(b model.Blockers) error {
		return apperr.Conflict("product is referenced by existing orders")
	}

	if atomic {
		for i, id := range ids {
			if id == "" {
				return bulk.Rollback(results, i, apperr.Validation("id is required")), nil
			}
			b, ok := blockers[id]
			if !ok {
				return bulk.Rollback(results, i, apperr.NotFound("product not found")), nil
			}
			if b.OrderItems > 0 {
				return bulk.Rollback(results, i, blocked(b)), nil
			}
		}
		if failIndex, err := s.repo.DeleteMany(ctx, ids); err != nil {
			if failIndex < 0 {
				failIndex = 0
			}
			return bulk.Rollback(results, failIndex, err), nil
		}
		for i, id := range ids {
			results[i] = bulk.OK[bulkdto.Empty](i, id, bulkdto.Empty{})
		}
		return results, nil
	}

	for i, id := range ids {
		b, ok := blockers[id]
		if !ok {
			continue // already marked NOT_FOUND
		}
		if b.OrderItems > 0 {
			results[i] = bulk.Failed[bulkdto.Empty](i, id, blocked(b))
			continue
		}
		if err := s.repo.Delete(ctx, id); err != nil {
			results[i] = bulk.Failed[bulkdto.Empty](i, id, err)
			continue
		}
		results[i] = bulk.OK[bulkdto.Empty](i, id, bulkdto.Empty{})
	}
	return results, nil
}

func blockMessages(b model.Blockers) []string {
	out := []string{}
	if b.OrderItems > 0 {
		out = append(out, fmt.Sprintf("referenced by %d orders", b.OrderItems))
	}
	if b.Variants > 0 {
		out = append(out, fmt.Sprintf("has %d variants", b.Variants))
	}
	if b.CartLines > 0 {
		out = append(out, fmt.Sprintf("in %d carts", b.CartLines))
	}
	if b.WishlistItems > 0 {
		out = append(out, fmt.Sprintf("in %d wishlists", b.WishlistItems))
	}
	return out
}

func (s *service) SetImage(ctx context.Context, id, url, publicID string) (public.Product, error) {
	if strings.TrimSpace(id) == "" {
		return public.Product{}, apperr.Validation("id is required")
	}
	if strings.TrimSpace(url) == "" {
		return public.Product{}, apperr.Validation("image url is required")
	}
	p, err := s.repo.SetImage(ctx, id, strings.TrimSpace(url), strings.TrimSpace(publicID))
	if err != nil {
		return public.Product{}, err
	}
	return toPublic(p), nil
}

// AddImage reserves a gallery slot before the bytes are uploaded. The row
// stays pending (empty url) until the media job's completion callback runs.
func (s *service) AddImage(ctx context.Context, productID string) (public.Image, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return public.Image{}, apperr.Validation("id is required")
	}
	p, err := s.repo.GetByID(ctx, productID)
	if err != nil {
		return public.Image{}, err
	}
	img, err := s.repo.CreateImage(ctx, productID)
	if err != nil {
		return public.Image{}, err
	}
	return toPublicImage(img, p), nil
}

// ListImages is the admin view: pending rows are included, so an operator
// can see uploads that are still queued.
func (s *service) ListImages(ctx context.Context, productID string) ([]public.Image, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, apperr.Validation("id is required")
	}
	p, err := s.repo.GetByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	images, err := s.repo.ListImages(ctx, productID)
	if err != nil {
		return nil, err
	}
	out := make([]public.Image, 0, len(images))
	for _, img := range images {
		out = append(out, toPublicImage(img, p))
	}
	return out, nil
}

func (s *service) DeleteImage(ctx context.Context, productID, imageID string) error {
	productID = strings.TrimSpace(productID)
	imageID = strings.TrimSpace(imageID)
	if productID == "" || imageID == "" {
		return apperr.Validation("id and image id are required")
	}
	return s.repo.DeleteImage(ctx, productID, imageID)
}

// SetPrimaryImage points the product's legacy image_url at one gallery row.
// Pending rows are rejected: there is no URL to promote yet.
func (s *service) SetPrimaryImage(ctx context.Context, productID, imageID string) (public.Product, error) {
	productID = strings.TrimSpace(productID)
	imageID = strings.TrimSpace(imageID)
	if productID == "" || imageID == "" {
		return public.Product{}, apperr.Validation("id and image id are required")
	}
	img, err := s.repo.GetImage(ctx, productID, imageID)
	if err != nil {
		return public.Product{}, err
	}
	if strings.TrimSpace(img.URL) == "" {
		return public.Product{}, apperr.Validation("image upload is not finished yet")
	}
	p, err := s.repo.SetPrimaryImage(ctx, productID, imageID)
	if err != nil {
		return public.Product{}, err
	}
	return toPublic(p), nil
}

// CompleteImage is the media pipeline callback. A row the admin deleted
// while the job was queued is a no-op (not a retryable error).
func (s *service) CompleteImage(ctx context.Context, imageID, url, publicID string) error {
	imageID = strings.TrimSpace(imageID)
	url = strings.TrimSpace(url)
	if imageID == "" {
		return apperr.Validation("image id is required")
	}
	if url == "" {
		return apperr.Validation("image url is required")
	}
	img, err := s.repo.CompleteImage(ctx, imageID, url, strings.TrimSpace(publicID))
	if err != nil {
		return err
	}
	if img.ProductID == "" {
		return nil // gallery row is gone
	}
	_, err = s.repo.PromoteIfEmpty(ctx, img.ProductID)
	return err
}
