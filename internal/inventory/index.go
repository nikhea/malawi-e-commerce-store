package inventory

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/inventory/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/inventory/repository"
	inventoryservice "github.com/nikhea/malawi-e-commerce-store/internal/inventory/service"
)

// Wire builds the inventory module: repository + service. Leaf of the
// fulfillment graph — orders and payments call this, it calls nobody.
// This file is the module's single entry point.
func Wire(pool *pgxpool.Pool) public.Service {
	return inventoryservice.NewService(repository.NewPostgres(pool))
}
