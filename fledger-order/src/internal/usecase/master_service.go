// Package usecase — master data services (Sprint 2): product, pricing, inventory.
package usecase

import (
	"context"
	"fmt"
	"strings"

	apperrors "github.com/fledger/fledger-order/internal/platform/errors"
	"github.com/fledger/fledger-order/internal/domain/inventory"
	"github.com/fledger/fledger-order/internal/domain/pricing"
	"github.com/fledger/fledger-order/internal/domain/product"
	"github.com/fledger/fledger-order/internal/repository/postgres"
)

// ProductService is the master-data service for FMCG products.
type ProductService struct {
	products *postgres.ProductRepo
}

func NewProductService(p *postgres.ProductRepo) *ProductService {
	return &ProductService{products: p}
}

type CreateProductInput struct {
	TenantID     string
	SKU          string
	Barcode      string
	Name         string
	Category     product.Category
	Unit         product.Unit
	WeightGrams  int
	Metadata     map[string]any
}

func (s *ProductService) Create(ctx context.Context, in CreateProductInput) (product.Product, error) {
	if strings.TrimSpace(in.SKU) == "" {
		return product.Product{}, fmt.Errorf("%w: sku required", apperrors.ErrInvalidInput)
	}
	if strings.TrimSpace(in.Name) == "" {
		return product.Product{}, fmt.Errorf("%w: name required", apperrors.ErrInvalidInput)
	}
	return s.products.Insert(ctx, product.Product{
		TenantID:    in.TenantID,
		SKU:         strings.TrimSpace(in.SKU),
		Barcode:     strings.TrimSpace(in.Barcode),
		Name:        strings.TrimSpace(in.Name),
		Category:    in.Category,
		Unit:        in.Unit,
		WeightGrams: in.WeightGrams,
		IsActive:    true,
		Metadata:    in.Metadata,
	})
}

func (s *ProductService) Get(ctx context.Context, tenantID, id string) (product.Product, error) {
	return s.products.Get(ctx, tenantID, id)
}

func (s *ProductService) List(ctx context.Context, tenantID, category string) ([]product.Product, error) {
	return s.products.List(ctx, tenantID, category)
}

// PricingService manages multi-tier prices.
type PricingService struct {
	prices *postgres.PricingRepo
}

func NewPricingService(p *postgres.PricingRepo) *PricingService { return &PricingService{prices: p} }

type CreatePricingInput struct {
	TenantID    string
	ProductID   string
	Tier        pricing.Tier
	MinQuantity int
	UnitPrice   int64
}

func (s *PricingService) Create(ctx context.Context, in CreatePricingInput) (pricing.PriceTier, error) {
	if !in.Tier.Valid() {
		return pricing.PriceTier{}, fmt.Errorf("%w: invalid tier %q", apperrors.ErrInvalidInput, in.Tier)
	}
	return s.prices.Insert(ctx, pricing.PriceTier{
		TenantID:    in.TenantID,
		ProductID:   in.ProductID,
		Tier:        in.Tier,
		MinQuantity: in.MinQuantity,
		UnitPrice:   in.UnitPrice,
	})
}

func (s *PricingService) FindBestPrice(ctx context.Context, tenantID, productID string, tier pricing.Tier, qty int) (pricing.PriceTier, error) {
	return s.prices.FindBestPrice(ctx, tenantID, productID, tier, qty)
}

func (s *PricingService) ListByProduct(ctx context.Context, productID string) ([]pricing.PriceTier, error) {
	return s.prices.ListByProduct(ctx, productID)
}

// InventoryService manages warehouse stock.
type InventoryService struct {
	inventory *postgres.InventoryRepo
}

func NewInventoryService(i *postgres.InventoryRepo) *InventoryService {
	return &InventoryService{inventory: i}
}

type AdjustStockInput struct {
	TenantID    string
	ProductID   string
	WarehouseID string
	Delta       int
}

func (s *InventoryService) Get(ctx context.Context, tenantID, productID, warehouseID string) (inventory.Stock, error) {
	return s.inventory.Get(ctx, tenantID, productID, warehouseID)
}

func (s *InventoryService) Adjust(ctx context.Context, in AdjustStockInput) (inventory.Stock, error) {
	return s.inventory.AdjustOnHand(ctx, in.TenantID, in.ProductID, in.WarehouseID, in.Delta)
}

func (s *InventoryService) Reserve(ctx context.Context, tenantID, productID, warehouseID string, qty int) (inventory.Stock, error) {
	return s.inventory.Reserve(ctx, tenantID, productID, warehouseID, qty)
}

func (s *InventoryService) Release(ctx context.Context, tenantID, productID, warehouseID string, qty int) (inventory.Stock, error) {
	return s.inventory.Release(ctx, tenantID, productID, warehouseID, qty)
}

func (s *InventoryService) ListByProduct(ctx context.Context, productID string) ([]inventory.Stock, error) {
	return s.inventory.ListByProduct(ctx, productID)
}