package users

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/users/repository"
	userservice "github.com/nikhea/malawi-e-commerce-store/internal/users/service"
)

// Wire builds the users module: repository + service, ready to serve and
// to inject into other modules. This file is the module's single entry
// point — cmd/* imports this package and calls Wire instead of assembling
// the layers by hand.
func Wire(pool *pgxpool.Pool) public.Service {
	return userservice.NewService(repository.NewPostgres(pool))
}
