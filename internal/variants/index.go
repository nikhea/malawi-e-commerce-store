package variants

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/repository"
	variantservice "github.com/nikhea/malawi-e-commerce-store/internal/variants/service"
)

// Wire builds the variants module: repository + service. Leaf of the
// catalog graph — imports no sibling. This file is the module's single
// entry point.
func Wire(pool *pgxpool.Pool) public.Service {
	return variantservice.NewService(repository.NewPostgres(pool))
}
