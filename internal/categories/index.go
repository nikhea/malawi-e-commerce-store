package categories

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/categories/public"
	categoriesrepo "github.com/nikhea/malawi-e-commerce-store/internal/categories/repository"
	categoriesservice "github.com/nikhea/malawi-e-commerce-store/internal/categories/service"
)

// Wire builds the categories module: repository + service, ready to serve
// and to inject into other modules (products reads this next). This file
// is the module's single entry point — cmd/* calls Wire instead of
// assembling the layers by hand.
func Wire(pool *pgxpool.Pool) public.Service {
	return categoriesservice.NewService(categoriesrepo.NewPostgres(pool))
}
