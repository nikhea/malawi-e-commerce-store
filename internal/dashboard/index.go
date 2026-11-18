package dashboard

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/dashboard/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/dashboard/repository"
	dashboardservice "github.com/nikhea/malawi-e-commerce-store/internal/dashboard/service"
)

// Wire builds the read-only dashboard module around the shared pgx pool.
// This is the module's single entry point; cmd/api never assembles its layers.
func Wire(pool *pgxpool.Pool) public.Service {
	return dashboardservice.NewService(repository.NewPostgres(pool))
}
