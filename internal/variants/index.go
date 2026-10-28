package variants

import (
	"github.com/jackc/pgx/v5/pgxpool"
	inventorypublic "github.com/nikhea/malawi-e-commerce-store/internal/inventory/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/repository"
	variantservice "github.com/nikhea/malawi-e-commerce-store/internal/variants/service"
)

// Wire builds the variants module: repository + service. Inventory is a
// lower-level sibling used only to hydrate per-SKU availability in reads.
// This file is the module's single entry point.
func Wire(pool *pgxpool.Pool, inventory inventorypublic.Service) public.Service {
	return variantservice.NewService(repository.NewPostgres(pool), inventory)
}
