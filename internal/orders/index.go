package orders

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	cartpublic "github.com/nikhea/malawi-e-commerce-store/internal/cart/public"
	inventorypublic "github.com/nikhea/malawi-e-commerce-store/internal/inventory/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/repository"
	orderservice "github.com/nikhea/malawi-e-commerce-store/internal/orders/service"
	"github.com/nikhea/malawi-e-commerce-store/pkg/events"
)

// Wire builds the orders module around cart (checkout source), inventory
// (stock holds), and the event bus (OrderCreated/Paid/Cancelled fan-out).
// This file is the module's single entry point.
func Wire(pool *pgxpool.Pool, cart cartpublic.Service, inventory inventorypublic.Service, bus *events.Bus) public.Service {
	return orderservice.NewService(
		repository.NewPostgres(pool), cart, inventory, bus,
		orderservice.Config{ReserveTTL: 15 * time.Minute},
	)
}
