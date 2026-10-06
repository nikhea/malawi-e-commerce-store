package reviews

import (
	"github.com/jackc/pgx/v5/pgxpool"
	productspublic "github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews/repository"
	reviewservice "github.com/nikhea/malawi-e-commerce-store/internal/reviews/service"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
)

// Wire builds the reviews module around products (existence) and users
// (owner verification + author snapshot). Leaf: catalog reads never call
// back — writes never block the read path. Single entry point.
func Wire(pool *pgxpool.Pool, products productspublic.Service, users userspublic.Service) public.Service {
	return reviewservice.NewService(repository.NewPostgres(pool), products, users)
}
