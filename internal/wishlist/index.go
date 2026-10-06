package wishlist

import (
	"github.com/jackc/pgx/v5/pgxpool"
	productspublic "github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/repository"
	wishlistservice "github.com/nikhea/malawi-e-commerce-store/internal/wishlist/service"
)

// Wire builds the wishlist module around products (existence + views)
// and users (owner verification). Leaf: nobody depends on wishlist.
// Single entry point.
func Wire(pool *pgxpool.Pool, products productspublic.Service, users userspublic.Service) public.Service {
	return wishlistservice.NewService(repository.NewPostgres(pool), products, users)
}
