package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-fleet/internal/platform/errors"
	"github.com/fledger/fledger-fleet/internal/domain/delivery_order"
	"github.com/fledger/fledger-fleet/internal/repository/postgres"
)

// DeliveryOrderService handles Surat Jalan lifecycle.
type DeliveryOrderService struct {
	repo *postgres.DORepo
	pool *pgxpool.Pool
}

func NewDeliveryOrderService(repo *postgres.DORepo, pool *pgxpool.Pool) *DeliveryOrderService {
	return &DeliveryOrderService{repo: repo, pool: pool}
}

// CreateInput bundles the input of POST /v1/fleet/delivery-orders.
type CreateInput struct {
	TenantID           string
	DoNumber           string
	CustomerID        string
	CustomerName      string
	DestinationAddress string
	DestinationLat    *float64
	DestinationLng    *float64
	Items             []delivery_order.DOItem
}

// Create validates inputs, computes the total nominal, and persists the DO.
func (s *DeliveryOrderService) Create(ctx context.Context, in CreateInput) (delivery_order.DeliveryOrder, []delivery_order.DOItem, error) {
	in.DoNumber = strings.TrimSpace(in.DoNumber)
	if in.DoNumber == "" {
		return delivery_order.DeliveryOrder{}, nil, fmt.Errorf("%w: do_number required", apperrors.ErrInvalidInput)
	}
	if in.CustomerID == "" {
		return delivery_order.DeliveryOrder{}, nil, fmt.Errorf("%w: customer_id required", apperrors.ErrInvalidInput)
	}
	if in.CustomerName == "" {
		return delivery_order.DeliveryOrder{}, nil, fmt.Errorf("%w: customer_name required", apperrors.ErrInvalidInput)
	}
	if in.DestinationAddress == "" {
		return delivery_order.DeliveryOrder{}, nil, fmt.Errorf("%w: destination_address required", apperrors.ErrInvalidInput)
	}
	if len(in.Items) == 0 {
		return delivery_order.DeliveryOrder{}, nil, fmt.Errorf("%w: at least one item required", apperrors.ErrInvalidInput)
	}
	totalOrdered := 0
	for i, it := range in.Items {
		if it.ProductSKU == "" {
			return delivery_order.DeliveryOrder{}, nil, fmt.Errorf("%w: items[%d].product_sku required", apperrors.ErrInvalidInput, i)
		}
		if it.QtyOrdered <= 0 {
			return delivery_order.DeliveryOrder{}, nil, fmt.Errorf("%w: items[%d].qty_ordered must be > 0", apperrors.ErrInvalidInput, i)
		}
		if it.UnitPriceCents <= 0 {
			return delivery_order.DeliveryOrder{}, nil, fmt.Errorf("%w: items[%d].unit_price_cents must be > 0", apperrors.ErrInvalidInput, i)
		}
		totalOrdered += it.QtyOrdered
	}

	d := delivery_order.DeliveryOrder{
		TenantID:            in.TenantID,
		DoNumber:            in.DoNumber,
		CustomerID:          in.CustomerID,
		CustomerName:        in.CustomerName,
		DestinationAddress:  in.DestinationAddress,
		DestinationLat:      in.DestinationLat,
		DestinationLng:      in.DestinationLng,
		TotalItemsOrdered:   totalOrdered,
		NominalOrderedCents: delivery_order.ComputeNominalOrdered(in.Items),
	}
	return s.repo.CreateDOWithItems(ctx, d, in.Items)
}

// Get returns one DO with its items.
func (s *DeliveryOrderService) Get(ctx context.Context, tenantID, id string) (delivery_order.DeliveryOrder, []delivery_order.DOItem, error) {
	d, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return delivery_order.DeliveryOrder{}, nil, err
	}
	items, err := s.repo.ListItems(ctx, id)
	if err != nil {
		return delivery_order.DeliveryOrder{}, nil, err
	}
	return d, items, nil
}

// List returns DOs filtered by status.
func (s *DeliveryOrderService) List(ctx context.Context, tenantID, status string) ([]delivery_order.DeliveryOrder, error) {
	if status != "" && !delivery_order.Status(status).Valid() {
		return nil, fmt.Errorf("%w: invalid status filter %q", apperrors.ErrInvalidInput, status)
	}
	return s.repo.List(ctx, tenantID, status)
}