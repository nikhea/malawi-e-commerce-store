package products

import (
	"github.com/jackc/pgx/v5/pgxpool"
	categoriespublic "github.com/nikhea/malawi-e-commerce-store/internal/categories/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/repository"
	productservice "github.com/nikhea/malawi-e-commerce-store/internal/products/service"
	variantspublic "github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
)

// Wire builds the products module around its siblings: categories for
// taxonomy validation, variants for detail composition. This file is the
// module's single entry point.
func Wire(pool *pgxpool.Pool, categories categoriespublic.Service, variants variantspublic.Service) public.Service {
	return productservice.NewService(repository.NewPostgres(pool), categories, variants)
}
