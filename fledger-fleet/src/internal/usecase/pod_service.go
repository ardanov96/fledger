package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fledger/fledger-fleet/internal/domain/delivery_order"
	"github.com/fledger/fledger-fleet/internal/domain/outbox"
	"github.com/fledger/fledger-fleet/internal/domain/pod"
	apperrors "github.com/fledger/fledger-fleet/internal/platform/errors"
	"github.com/fledger/fledger-fleet/internal/integration/coreclient"
	"github.com/fledger/fledger-fleet/internal/repository/postgres"
)

// ProofOfDeliveryService persists PODs and drives the auto-invoicing pipeline.
type ProofOfDeliveryService struct {
	pods       *postgres.PODRepo
	dos        *postgres.DORepo
	outbox     *postgres.OutboxRepo
	audit      *postgres.AuditRepo
	coreClient *coreclient.Client
	pool       *pgxpool.Pool
}

func NewProofOfDeliveryService(
	pods *postgres.PODRepo,
	dos *postgres.DORepo,
	ob *postgres.OutboxRepo,
	audit *postgres.AuditRepo,
	core *coreclient.Client,
	pool *pgxpool.Pool,
) *ProofOfDeliveryService {
	return &ProofOfDeliveryService{pods: pods, dos: dos, outbox: ob, audit: audit, coreClient: core, pool: pool}
}

// SubmitInput is the shape of POST /v1/fleet/delivery-orders/:id/pod.
type SubmitInput struct {
	TenantID          string
	DOID              string
	ActorID           string // §7.3 — required for the audit trail row
	RecipientName     string
	RecipientPhone    string
	SignatureDataURL  string
	PhotoEvidenceURLs []string
	DeliveredLat      float64
	DeliveredLng      float64
	DriverNotes       string
	Items             []pod.ItemResult
}

// SubmitResult is what the use case returns to the handler.
type SubmitResult struct {
	DO                  delivery_order.DeliveryOrder `json:"do"`
	Status              string                       `json:"status"`
	NominalDeliveredCents int64                      `json:"nominal_delivered_cents"`
	FledgerInvoiceID    string                       `json:"fledger_invoice_id,omitempty"`
	Queued              bool                         `json:"queued"`
	Message             string                       `json:"message"`
}

// Submit validates the POD against the current DO, applies the result, fires
// the invoice sync (synchronously if Core is reachable, otherwise queues to
// the outbox), and returns a result with the new status + invoice id.
func (s *ProofOfDeliveryService) Submit(ctx context.Context, in SubmitInput) (*SubmitResult, error) {
	if in.DOID == "" {
		return nil, fmt.Errorf("%w: do_id required", apperrors.ErrInvalidInput)
	}
	if in.RecipientName == "" {
		return nil, fmt.Errorf("%w: recipient_name required", apperrors.ErrInvalidInput)
	}
	if in.SignatureDataURL == "" {
		return nil, fmt.Errorf("%w: signature_data_url required", apperrors.ErrInvalidInput)
	}
	if len(in.Items) == 0 {
		return nil, fmt.Errorf("%w: items result required", apperrors.ErrInvalidInput)
	}

	do, err := s.dos.Get(ctx, in.TenantID, in.DOID)
	if err != nil {
		return nil, err
	}
	if do.Status != delivery_order.StatusOutForDelivery {
		return nil, fmt.Errorf("%w: DO must be OUT_FOR_DELIVERY before submitting POD (current=%s)",
			apperrors.ErrInvalidInput, do.Status)
	}
	items, err := s.dos.ListItems(ctx, in.DOID)
	if err != nil {
		return nil, err
	}

	// Build a SKU map from the current items for validation.
	ordered := make(map[string]delivery_order.DOItem, len(items))
	for _, it := range items {
		ordered[it.ProductSKU] = it
	}

	totalDelivered, totalRejected, nominalDelivered := 0, 0, int64(0)
	itemUpdates := make(map[string]struct {
		QtyDelivered    int
		QtyRejected     int
		RejectionReason string
	}, len(in.Items))
	for _, r := range in.Items {
		base, ok := ordered[r.ProductSKU]
		if !ok {
			return nil, fmt.Errorf("%w: unknown SKU %s", apperrors.ErrInvalidInput, r.ProductSKU)
		}
		if r.QtyDelivered < 0 || r.QtyRejected < 0 {
			return nil, fmt.Errorf("%w: qty must be >= 0 for %s", apperrors.ErrInvalidInput, r.ProductSKU)
		}
		if r.QtyDelivered+r.QtyRejected != base.QtyOrdered {
			return nil, fmt.Errorf("%w: %s delivered(%d)+rejected(%d) must equal ordered(%d)",
				apperrors.ErrInvalidInput, r.ProductSKU, r.QtyDelivered, r.QtyRejected, base.QtyOrdered)
		}
		// Sprint 3: if any qty_rejected > 0 the driver MUST provide:
		//   - a photo_evidence_urls entry
		//   - a rejection_reason
		if r.QtyRejected > 0 {
			if r.RejectionReason == "" {
				return nil, fmt.Errorf("%w: rejection_reason required when qty_rejected > 0 for %s",
					apperrors.ErrInvalidInput, r.ProductSKU)
			}
			if _, ok := pod.ValidRejectionReasons()[r.RejectionReason]; !ok {
				return nil, fmt.Errorf("%w: unknown rejection_reason %q for %s",
					apperrors.ErrInvalidInput, r.RejectionReason, r.ProductSKU)
			}
			if len(in.PhotoEvidenceURLs) == 0 {
				return nil, fmt.Errorf("%w: photo_evidence_urls required when any qty_rejected > 0",
					apperrors.ErrInvalidInput)
			}
		}
		totalDelivered += r.QtyDelivered
		totalRejected += r.QtyRejected
		nominalDelivered += int64(r.QtyDelivered) * base.UnitPriceCents
		itemUpdates[r.ProductSKU] = struct {
			QtyDelivered    int
			QtyRejected     int
			RejectionReason string
		}{r.QtyDelivered, r.QtyRejected, r.RejectionReason}
	}

	newStatus := pod.ComputeDOStatus(do.TotalItemsOrdered, totalDelivered, totalRejected)

	// Persist POD + DO update first; invoice id is added in a second pass.
	proof := pod.ProofOfDelivery{
		DOID:              do.ID,
		RecipientName:     in.RecipientName,
		RecipientPhone:    in.RecipientPhone,
		SignatureDataURL:  in.SignatureDataURL,
		PhotoEvidenceURLs: in.PhotoEvidenceURLs,
		DeliveredLat:      in.DeliveredLat,
		DeliveredLng:      in.DeliveredLng,
		DriverNotes:       in.DriverNotes,
		Items:             in.Items,
	}
	if _, err := s.pods.Insert(ctx, proof); err != nil {
		return nil, err
	}

	updated, err := s.dos.ApplyPODResult(
		ctx, in.TenantID, do.ID, newStatus,
		totalDelivered, totalRejected, nominalDelivered, itemUpdates, nil,
	)
	if err != nil {
		return nil, err
	}

	// §7.3 — append audit-trail row for the status transition + set actor_id.
	if s.audit != nil {
		from := string(do.Status)
		to := string(newStatus)
		note := fmt.Sprintf("POD submitted (delivered %d, rejected %d)", totalDelivered, totalRejected)
		_ = s.audit.Append(ctx, in.TenantID, do.ID, from, to, in.ActorID, "driver", note)
		_ = s.audit.SetActor(ctx, in.TenantID, do.ID, in.ActorID)
	}

	// Build the outbox payload (so retries reuse the same invoice number).
	due := time.Now().UTC().Add(14 * 24 * time.Hour).Format("2006-01-02")
	payload := map[string]any{
		"customer_id":       do.CustomerID,
		"code":              "INV-DO-" + do.DoNumber,
		"amount_minor":      nominalDelivered,
		"due_date":          due,
		"description":       fmt.Sprintf("Generated from %s (delivered %d, rejected %d) by %s", do.DoNumber, totalDelivered, totalRejected, in.RecipientName),
		"do_id":             do.ID,
		"do_number":         do.DoNumber,
		"delivered_recipient": in.RecipientName,
	}
	if _, err := s.outbox.Append(ctx, outbox.Event{
		TenantID:      in.TenantID,
		AggregateType: "DELIVERY_ORDER",
		AggregateID:   do.ID,
		EventType:     "DO_POD_SUBMITTED",
		Subject:       "fledger.fleet.do.pod_submitted",
		Payload:       payload,
	}); err != nil {
		return nil, fmt.Errorf("enqueue outbox: %w", err)
	}

	// Try synchronous settle. On upstream failure we return queued=true so the
	// handler can respond 200 OK without blocking on a retry.
	res := &SubmitResult{
		DO:                   updated,
		Status:               string(newStatus),
		NominalDeliveredCents: nominalDelivered,
		Queued:               true,
		Message:              "POD stored; invoice queued for sync to Fledger Core",
	}

	invoice, err := s.coreClient.CreateInvoice(ctx, coreclient.InvoiceInput{
		CustomerID:  do.CustomerID,
		Code:        payload["code"].(string),
		AmountMinor: nominalDelivered,
		DueDate:     due,
		Description: payload["description"].(string),
		// Idempotency-Key per §5.1 of the brief: DO UUID, so any retry of the
		// same POD (including outbox worker re-tries) collapses to a single
		// invoice in Core.
		IdempotencyKey: do.ID,
		Metadata: map[string]any{
			"do_id":        do.ID,
			"do_number":    do.DoNumber,
			"fleet_source": "fledger-fleet",
		},
	})
	if err != nil {
		// Upstream unavailable or 4xx — leave queued=true, worker retries.
		return updatedAndQueue(res, updated, nominalDelivered), nil
	}

	// Update DO with fledger_invoice_id.
	withInvoice, err := s.dos.ApplyPODResult(
		ctx, in.TenantID, do.ID, newStatus,
		totalDelivered, totalRejected, nominalDelivered, itemUpdates, &invoice.ID,
	)
	if err != nil {
		return nil, err
	}
	res.DO = withInvoice
	res.FledgerInvoiceID = invoice.ID
	res.Queued = false
	res.Message = "POD successfully submitted and clean invoice synced to Fledger Core"
	return res, nil
}

// updatedAndQueue is a tiny helper used when sync settle fails but we still
// need to attach the latest DO + nominal to the response.
func updatedAndQueue(r *SubmitResult, do delivery_order.DeliveryOrder, nominal int64) *SubmitResult {
	r.DO = do
	r.NominalDeliveredCents = nominal
	return r
}

// MarshalPayload is exported so the outbox worker can re-encode the stored
// payload into the live request body (avoids drift between write & read).
func MarshalPayload(e outbox.Event) ([]byte, error) { return json.Marshal(e.Payload) }