package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-fleet/internal/platform/errors"
	"github.com/fledger/fledger-fleet/internal/domain/pod"
)

// PODRepo persists Proof of Delivery rows.
type PODRepo struct {
	pool *pgxpool.Pool
}

func NewPODRepo(p *pgxpool.Pool) *PODRepo { return &PODRepo{pool: p} }

// Insert writes a new POD row. The DOID has a UNIQUE index so a second POD
// for the same DO returns ErrConflict.
func (r *PODRepo) Insert(ctx context.Context, p pod.ProofOfDelivery) (pod.ProofOfDelivery, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO fleet_proof_of_deliveries
		  (do_id, recipient_name, recipient_phone, signature_data_url,
		   photo_evidence_urls, delivered_lat, delivered_lng, driver_notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, do_id, recipient_name, recipient_phone, signature_data_url,
		          photo_evidence_urls, delivered_lat, delivered_lng, delivered_at,
		          driver_notes, created_at`,
		p.DOID, p.RecipientName, p.RecipientPhone, p.SignatureDataURL,
		p.PhotoEvidenceURLs, p.DeliveredLat, p.DeliveredLng, p.DriverNotes,
	)
	var got pod.ProofOfDelivery
	if err := row.Scan(
		&got.ID, &got.DOID, &got.RecipientName, &got.RecipientPhone, &got.SignatureDataURL,
		&got.PhotoEvidenceURLs, &got.DeliveredLat, &got.DeliveredLng, &got.DeliveredAt,
		&got.DriverNotes, &got.CreatedAt,
	); err != nil {
		if isUniqueViolation(err) {
			return pod.ProofOfDelivery{}, fmt.Errorf("%w: POD already submitted", apperrors.ErrConflict)
		}
		return pod.ProofOfDelivery{}, fmt.Errorf("insert POD: %w", err)
	}
	got.Items = p.Items
	return got, nil
}

// Get fetches the POD attached to a DO (returns ErrNotFound when missing).
func (r *PODRepo) GetByDO(ctx context.Context, doID string) (pod.ProofOfDelivery, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, do_id, recipient_name, recipient_phone, signature_data_url,
		       photo_evidence_urls, delivered_lat, delivered_lng, delivered_at,
		       driver_notes, created_at
		  FROM fleet_proof_of_deliveries WHERE do_id = $1`,
		doID,
	)
	var got pod.ProofOfDelivery
	if err := row.Scan(
		&got.ID, &got.DOID, &got.RecipientName, &got.RecipientPhone, &got.SignatureDataURL,
		&got.PhotoEvidenceURLs, &got.DeliveredLat, &got.DeliveredLng, &got.DeliveredAt,
		&got.DriverNotes, &got.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pod.ProofOfDelivery{}, fmt.Errorf("%w: POD for DO %s", apperrors.ErrNotFound, doID)
		}
		return pod.ProofOfDelivery{}, err
	}
	return got, nil
}