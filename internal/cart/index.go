package cart

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/cart/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/cart/repository"
	cartservice "github.com/nikhea/malawi-e-commerce-store/internal/cart/service"
	productspublic "github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
)

// Wire builds the cart module around its siblings: products for catalog
// validation + price snapshots, users for owner verification. This file
// is the module's single entry point.
func Wire(pool *pgxpool.Pool, products productspublic.Service, users userspublic.Service) public.Service {
	return cartservice.NewService(repository.NewPostgres(pool), products, users)
}
