//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	"golang.org/x/crypto/bcrypt"
)

const (
	tenantID = "00000000-0000-0000-0000-000000000001"
	adminID  = "33333333-3333-3333-3333-333333333333"
	salesID  = "44444444-4444-4444-4444-444444444444"

	// Periods
	periodSepID = "77777777-7777-7777-7777-000000000009"
	periodOctID = "77777777-7777-7777-7777-777777777777"

	// Cash & Operating Accounts
	accCashHQ   = "aaaaaaaa-0001-0000-0000-000000000001"
	accBankBCA  = "aaaaaaaa-0002-0000-0000-000000000002"
	accBankMand = "aaaaaaaa-0006-0000-0000-000000000006"
	accCashJoni = "aaaaaaaa-0007-0000-0000-000000000007"
	accCashRian = "aaaaaaaa-0008-0000-0000-000000000008"

	// Nominal / Control Accounts
	accARCtrl = "aaaaaaaa-0003-0000-0000-000000000003"
	accSalesRev = "aaaaaaaa-0004-0000-0000-000000000004"
	accOpExp    = "aaaaaaaa-0005-0000-0000-000000000005"

	// Customer Accounts
	custTokoA   = "cccccccc-0001-0000-0000-000000000001"
	custWarungB = "cccccccc-0002-0000-0000-000000000002"
	custPrimaC  = "cccccccc-0003-0000-0000-000000000003"
	custSentosaD= "cccccccc-0004-0000-0000-000000000004"
	custMaduraE = "cccccccc-0005-0000-0000-000000000005"

	// Invoices
	inv101 = "10000001-0101-0000-0000-000000000001"
	inv089 = "10000001-0089-0000-0000-000000000002"
	inv102 = "10000002-0102-0000-0000-000000000003"
	inv095 = "10000003-0095-0000-0000-000000000004"
	inv098 = "10000004-0098-0000-0000-000000000005"
	inv105 = "10000005-0105-0000-0000-000000000006"
	inv072 = "10000005-0072-0000-0000-000000000007"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://fmcg:fmcg_dev_password@localhost:5432/fmcg_wallet?sslmode=disable"
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer conn.Close(ctx)

	log.Println("=== Seeding Rich Connected Dummy Data for FMCG Wallet ===")

	// 1. Currencies
	_, err = conn.Exec(ctx, `
		INSERT INTO currencies (code, name, decimal_places, is_active)
		VALUES 
			('IDR', 'Indonesian Rupiah', 2, TRUE),
			('USD', 'US Dollar', 2, TRUE),
			('SGD', 'Singapore Dollar', 2, TRUE)
		ON CONFLICT (code) DO UPDATE SET is_active = TRUE;
	`)
	if err != nil {
		log.Fatalf("currencies: %v", err)
	}
	log.Println("✓ Currencies seeded (IDR, USD, SGD)")

	// 2. Demo User Credentials (Password: DemoTest1234!)
	pwHash, _ := bcrypt.GenerateFromPassword([]byte("DemoTest1234!"), 10)
	_, err = conn.Exec(ctx, `
		INSERT INTO user_credentials (user_id, tenant_id, password_hash, mfa_enabled, failed_login_count, locked_until)
		VALUES
			($1, $2, $3, FALSE, 0, NULL),
			($4, $2, $3, FALSE, 0, NULL)
		ON CONFLICT (user_id) DO UPDATE SET password_hash = EXCLUDED.password_hash;
	`, adminID, tenantID, string(pwHash), salesID)
	if err != nil {
		log.Fatalf("user_credentials: %v", err)
	}
	log.Println("✓ User credentials seeded (admin & sales)")

	// 3. Accounting Periods (September closed, October open)
	// Update October period first to prevent overlapping exclusion constraint with existing row
	_, _ = conn.Exec(ctx, `
		UPDATE accounting_periods 
		SET period_start = '2026-10-01', period_end = '2026-10-31', status = 'open' 
		WHERE id = $1;
	`, periodOctID)

	_, err = conn.Exec(ctx, `
		INSERT INTO accounting_periods (id, tenant_id, period_start, period_end, status, closed_at, closed_by)
		VALUES
			($1, $3, '2026-09-01', '2026-09-30', 'closed', '2026-10-01 00:00:00Z', $4),
			($2, $3, '2026-10-01', '2026-10-31', 'open', NULL, NULL)
		ON CONFLICT (id) DO UPDATE SET 
			status = EXCLUDED.status,
			period_start = EXCLUDED.period_start,
			period_end = EXCLUDED.period_end,
			closed_at = EXCLUDED.closed_at;
	`, periodSepID, periodOctID, tenantID, adminID)
	if err != nil {
		log.Fatalf("accounting_periods: %v", err)
	}
	log.Println("✓ Accounting periods seeded (Sept Closed, Oct Open)")

	// 4. Accounts (Cash, Banks, AR, Revenue, Expense, Customers)
	accounts := []struct {
		id, code, name, accType string
		balance                int64
		metadata               string
	}{
		{accCashHQ, "CASH-001", "Cash on Hand HQ", "cash", 15000000000, `{"location":"HQ Jakarta"}`},
		{accBankBCA, "BANK-BCA", "Bank BCA Operasional", "cash", 85000000000, `{"account_no":"8820192341"}`},
		{accBankMand, "BANK-MANDIRI", "Bank Mandiri Penerimaan", "cash", 42000000000, `{"account_no":"1240098765"}`},
		{accCashJoni, "CASH-JONI", "Kas Sales Rep Joni", "cash", 2500000000, `{"sales_rep":"Joni Hermanto","area":"Jakarta Barat"}`},
		{accCashRian, "CASH-RIAN", "Kas Sales Rep Rian", "cash", 1200000000, `{"sales_rep":"Rian Hidayat","area":"Jakarta Selatan"}`},
		{accARCtrl, "AR-CTRL", "AR Control (Receivables)", "receivable", 10350000000, `{}`},
		{accSalesRev, "SALES-REV", "Sales Revenue", "revenue", 155000000000, `{}`},
		{accOpExp, "OP-EXP", "Operating Expense HQ", "hq", 4500000000, `{}`},
		{custTokoA, "CUST-TOKO-A", "Toko Sumber Rezeki", "customer", -4350000000, `{"owner":"Pak Ahmad","address":"Jl. Daan Mogot No. 12"}`},
		{custWarungB, "CUST-WARUNG-B", "Warung Berkah Jaya", "customer", -500000000, `{"owner":"Bu Siti","address":"Jl. Tebet Raya No. 45"}`},
		{custPrimaC, "CUST-PRIMA-C", "Minimarket Prima Mart", "customer", 0, `{"owner":"Pak Budi","address":"Jl. Raya Serpong No. 8"}`},
		{custSentosaD, "CUST-SENTOSA-D", "Agen Sentosa Abadi", "customer", -3500000000, `{"owner":"Ko Hendra","address":"Jl. Ahmad Yani No. 99"}`},
		{custMaduraE, "CUST-MADURA-E", "Warung Madura 24 Jam", "customer", -2300000000, `{"owner":"Cak Rohim","address":"Jl. Margonda Raya No. 17"}`},
	}

	for _, a := range accounts {
		_, err = conn.Exec(ctx, `
			INSERT INTO accounts (id, tenant_id, code, name, type, status, currency, cached_balance, metadata)
			VALUES ($1, $2, $3, $4, $5, 'active', 'IDR', $6, $7::jsonb)
			ON CONFLICT (id) DO UPDATE SET
				code = EXCLUDED.code,
				name = EXCLUDED.name,
				type = EXCLUDED.type,
				cached_balance = EXCLUDED.cached_balance,
				metadata = EXCLUDED.metadata;
		`, a.id, tenantID, a.code, a.name, a.accType, a.balance, a.metadata)
		if err != nil {
			log.Fatalf("account %s: %v", a.code, err)
		}
	}
	log.Printf("✓ %d Accounts seeded and updated", len(accounts))

	// 5. FX Rates
	_, _ = conn.Exec(ctx, `DELETE FROM fx_rates WHERE tenant_id = $1`, tenantID)
	_, err = conn.Exec(ctx, `
		INSERT INTO fx_rates (id, tenant_id, from_currency, to_currency, rate, source, effective_at, expires_at, created_by)
		VALUES
			(gen_random_uuid(), $1, 'USD', 'IDR', 15750.00, 'Bank Indonesia', NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', $2),
			(gen_random_uuid(), $1, 'IDR', 'USD', 0.000063492063492, 'Bank Indonesia', NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', $2),
			(gen_random_uuid(), $1, 'SGD', 'IDR', 11850.00, 'Bank Indonesia', NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', $2),
			(gen_random_uuid(), $1, 'IDR', 'SGD', 0.000084388185654, 'Bank Indonesia', NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', $2)
	`, tenantID, adminID)
	if err != nil {
		log.Fatalf("fx_rates: %v", err)
	}
	log.Println("✓ FX Rates seeded (USD/IDR, SGD/IDR)")

	// 6. Invoices across all Aging Buckets
	_, _ = conn.Exec(ctx, `DELETE FROM invoice_payments WHERE tenant_id = $1`, tenantID)
	_, _ = conn.Exec(ctx, `DELETE FROM invoices WHERE tenant_id = $1`, tenantID)

	invoices := []struct {
		id, custID, code string
		amount, paid     int64
		dueDays          int
		status, desc     string
		periodID         string
	}{
		{inv101, custTokoA, "INV-2026-0101", 2500000000, 0, 14, "open", "PO #8812 - 50 Dus Minyak Goreng Bimoli 2L & 20 Karung Beras Rojo", periodOctID},
		{inv089, custTokoA, "INV-2026-0089", 1850000000, 0, -35, "overdue", "PO #8140 - 100 Dus Gula Pasir Gulaku & Tepung Segitiga Biru", periodSepID},
		{inv102, custWarungB, "INV-2026-0102", 1200000000, 700000000, -5, "partial", "PO #8855 - Paket Sabun Mandi Lifebuoy & Deterjen Rinso Matic", periodOctID},
		{inv095, custPrimaC, "INV-2026-0095", 5000000000, 5000000000, -15, "paid", "PO #8720 - 150 Karton Minuman Teh Botol & Biskuit Khong Guan", periodSepID},
		{inv098, custSentosaD, "INV-2026-0098", 3500000000, 0, -20, "overdue", "PO #8790 - 200 Dus Indomie Goreng & Susu Kental Manis Frisian Flag", periodSepID},
		{inv105, custMaduraE, "INV-2026-0105", 1500000000, 0, 25, "open", "PO #8901 - 30 Slop Rokok Sampoerna Mild & Kopi Kapal Api Mix", periodOctID},
		{inv072, custMaduraE, "INV-2026-0072", 800000000, 0, -95, "overdue", "PO #7990 - Pengadaan Etalase & Minuman Isotonik Mizone", periodSepID},
	}

	now := time.Now().UTC()
	for _, inv := range invoices {
		dueDate := now.AddDate(0, 0, inv.dueDays)
		_, err = conn.Exec(ctx, `
			INSERT INTO invoices (id, tenant_id, customer_id, code, amount, paid_amount, due_date, status, period_id, description)
			VALUES ($1, $2, $3, $4, $5, 0, $6, 'open', $7, $8)
		`, inv.id, tenantID, inv.custID, inv.code, inv.amount, dueDate, inv.periodID, inv.desc)
		if err != nil {
			log.Fatalf("invoice %s: %v", inv.code, err)
		}
	}
	log.Printf("✓ %d Invoices seeded", len(invoices))

	// 7. Invoice Payments (Triggers will adjust paid_amount & status automatically)
	payments := []struct {
		invID, custID, method, note string
		amount                       int64
		daysAgo                      int
	}{
		{inv102, custWarungB, "transfer", "Cicilan termin 1 via transfer BCA", 700000000, 3},
		{inv095, custPrimaC, "transfer", "Pelunasan penuh PO #8720 via transfer BCA", 5000000000, 15},
	}

	for _, p := range payments {
		paymentID := uuid.NewString()
		allocAt := now.AddDate(0, 0, -p.daysAgo)
		_, err = conn.Exec(ctx, `
			INSERT INTO invoice_payments (id, tenant_id, payment_id, invoice_id, customer_id, amount, method, allocated_at, metadata)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, json_build_object('note', $8::text))
		`, tenantID, paymentID, p.invID, p.custID, p.amount, p.method, allocAt, p.note)
		if err != nil {
			log.Fatalf("payment for %s: %v", p.invID, err)
		}
	}
	log.Printf("✓ %d Invoice Payments & Allocations recorded", len(payments))

	// Mark overdue invoices where due_date < now
	_, _ = conn.Exec(ctx, `
		UPDATE invoices SET status = 'overdue' 
		WHERE tenant_id = $1 AND due_date < CURRENT_DATE AND status = 'open';
	`, tenantID)

	// 8. Aging Snapshots
	runID := uuid.New()
	_, _ = conn.Exec(ctx, `DELETE FROM aging_snapshots WHERE tenant_id = $1`, tenantID)
	agingRows := []struct {
		custID, bucket string
		count          int
		amount         int64
	}{
		{custTokoA, "current", 1, 2500000000},
		{custTokoA, "d_31_60", 1, 1850000000},
		{custWarungB, "d_1_7", 1, 500000000},
		{custSentosaD, "d_8_30", 1, 3500000000},
		{custMaduraE, "current", 1, 1500000000},
		{custMaduraE, "d_90_plus", 1, 800000000},
	}
	for _, ar := range agingRows {
		_, err = conn.Exec(ctx, `
			INSERT INTO aging_snapshots (tenant_id, customer_id, bucket, invoice_count, outstanding_minor, snapshot_at, snapshot_run_id)
			VALUES ($1, $2, $3, $4, $5, NOW(), $6)
		`, tenantID, ar.custID, ar.bucket, ar.count, ar.amount, runID)
		if err != nil {
			log.Printf("aging snapshot warning: %v", err)
		}
	}
	log.Println("✓ Aging snapshots populated")

	// 9. Interconnected Double-Entry Transactions & Ledger Entries
	type TxnSeed struct {
		id, idemKey, desc, fromAcc, toAcc string
		amount                            int64
		periodID                          string
		daysAgo                           int
	}

	txns := []TxnSeed{
		{
			id:       "90000001-0001-0000-0000-000000000001",
			idemKey:  "idem-seed-001",
			desc:     "Setoran tunai hasil penagihan toko Sales Joni area Jkt Barat",
			fromAcc:  accCashJoni,
			toAcc:    accBankBCA,
			amount:   2000000000, // Rp 20.000.000
			periodID: periodOctID,
			daysAgo:  2,
		},
		{
			id:       "90000002-0002-0000-0000-000000000002",
			idemKey:  "idem-seed-002",
			desc:     "Pencairan kas kecil operasional mingguan HQ",
			fromAcc:  accBankBCA,
			toAcc:    accCashHQ,
			amount:   1500000000, // Rp 15.000.000
			periodID: periodOctID,
			daysAgo:  3,
		},
		{
			id:       "90000003-0003-0000-0000-000000000003",
			idemKey:  "idem-seed-003",
			desc:     "Dropping uang jalan & bensin armada Sales Rian",
			fromAcc:  accCashHQ,
			toAcc:    accCashRian,
			amount:   500000000, // Rp 5.000.000
			periodID: periodOctID,
			daysAgo:  4,
		},
		{
			id:       "90000004-0004-0000-0000-000000000004",
			idemKey:  "idem-seed-004",
			desc:     "Konsolidasi penerimaan setoran harian Mandiri ke BCA Operasional",
			fromAcc:  accBankMand,
			toAcc:    accBankBCA,
			amount:   5000000000, // Rp 50.000.000
			periodID: periodOctID,
			daysAgo:  5,
		},
		{
			id:       "90000005-0005-0000-0000-000000000005",
			idemKey:  "idem-seed-005",
			desc:     "Penerimaan pelunasan faktur INV-2026-0095 Minimarket Prima Mart",
			fromAcc:  accARCtrl,
			toAcc:    accBankBCA,
			amount:   5000000000, // Rp 50.000.000
			periodID: periodSepID,
			daysAgo:  15,
		},
		{
			id:       "90000006-0006-0000-0000-000000000006",
			idemKey:  "idem-seed-006",
			desc:     "Penerimaan cicilan faktur INV-2026-0102 Warung Berkah Jaya",
			fromAcc:  accARCtrl,
			toAcc:    accBankBCA,
			amount:   700000000, // Rp 7.000.000
			periodID: periodOctID,
			daysAgo:  3,
		},
	}

	// We compute previous hashes per account
	accountLastHash := make(map[string]string)

	for _, t := range txns {
		createdAt := now.AddDate(0, 0, -t.daysAgo)
		_, err = conn.Exec(ctx, `
			INSERT INTO transactions (id, idempotency_key, status, description, initiator_id, tenant_id, period_id, posted_at, created_at, updated_at)
			VALUES ($1, $2, 'posted', $3, $4, $5, $6, $7, $7, $7)
			ON CONFLICT (tenant_id, idempotency_key) DO NOTHING;
		`, t.id, t.idemKey, t.desc, adminID, tenantID, t.periodID, createdAt)
		if err != nil {
			log.Fatalf("transaction %s: %v", t.idemKey, err)
		}

		// Entry 1: Debit toAcc
		prevHashTo := accountLastHash[t.toAcc]
		if prevHashTo == "" {
			prevHashTo = ledger.ZeroHash
		}
		hashInTo := ledger.HashInput{
			PrevHash:      prevHashTo,
			AccountID:     t.toAcc,
			TransactionID: t.id,
			PeriodID:      t.periodID,
			Type:          ledger.EntryTypeDebit,
			AmountMinor:   t.amount,
			Currency:      "IDR",
			Description:   t.desc,
			CreatedAt:     createdAt,
		}
		entryHashTo, err := ledger.ComputeHash(hashInTo)
		if err != nil {
			log.Fatalf("compute hash debit: %v", err)
		}
		accountLastHash[t.toAcc] = entryHashTo

		_, err = conn.Exec(ctx, `
			INSERT INTO ledger_entries (id, transaction_id, account_id, amount, type, period_id, description, currency, prev_hash, entry_hash, created_at)
			VALUES (gen_random_uuid(), $1, $2, $3, 'debit', $4, $5, 'IDR', $6, $7, $8)
		`, t.id, t.toAcc, t.amount, t.periodID, t.desc, prevHashTo, entryHashTo, createdAt)
		if err != nil {
			log.Fatalf("entry debit for txn %s: %v", t.id, err)
		}

		// Entry 2: Credit fromAcc
		prevHashFrom := accountLastHash[t.fromAcc]
		if prevHashFrom == "" {
			prevHashFrom = ledger.ZeroHash
		}
		hashInFrom := ledger.HashInput{
			PrevHash:      prevHashFrom,
			AccountID:     t.fromAcc,
			TransactionID: t.id,
			PeriodID:      t.periodID,
			Type:          ledger.EntryTypeCredit,
			AmountMinor:   t.amount,
			Currency:      "IDR",
			Description:   t.desc,
			CreatedAt:     createdAt,
		}
		entryHashFrom, err := ledger.ComputeHash(hashInFrom)
		if err != nil {
			log.Fatalf("compute hash credit: %v", err)
		}
		accountLastHash[t.fromAcc] = entryHashFrom

		_, err = conn.Exec(ctx, `
			INSERT INTO ledger_entries (id, transaction_id, account_id, amount, type, period_id, description, currency, prev_hash, entry_hash, created_at)
			VALUES (gen_random_uuid(), $1, $2, $3, 'credit', $4, $5, 'IDR', $6, $7, $8)
		`, t.id, t.fromAcc, t.amount, t.periodID, t.desc, prevHashFrom, entryHashFrom, createdAt)
		if err != nil {
			log.Fatalf("entry credit for txn %s: %v", t.id, err)
		}
	}
	log.Printf("✓ %d Transactions & %d Double-entry Ledger Entries created with valid SHA-256 Hash Chain", len(txns), len(txns)*2)

	// 10. Reconciler Runs
	_, _ = conn.Exec(ctx, `DELETE FROM reconciler_runs WHERE tenant_id = $1`, tenantID)
	_, err = conn.Exec(ctx, `
		INSERT INTO reconciler_runs (id, tenant_id, period_id, started_at, finished_at, status, total_debit, total_credit, imbalance, hash_chain_ok, hash_chain_errors, triggered_by)
		VALUES
			(gen_random_uuid(), $1, $2, NOW() - INTERVAL '7 days', NOW() - INTERVAL '7 days' + INTERVAL '2 seconds', 'balanced', 5000000000, 5000000000, 0, TRUE, 0, 'period_close'),
			(gen_random_uuid(), $1, $3, NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour' + INTERVAL '1 second', 'balanced', 9700000000, 9700000000, 0, TRUE, 0, 'scheduler');
	`, tenantID, periodSepID, periodOctID)
	if err != nil {
		log.Fatalf("reconciler_runs: %v", err)
	}
	log.Println("✓ Reconciler runs recorded (Status: Balanced)")

	// 11. Audit Logs
	_, _ = conn.Exec(ctx, `DELETE FROM audit_logs WHERE tenant_id = $1`, tenantID)
	auditEvents := []struct {
		action, resType, resID, outcome string
		meta                            string
		hoursAgo                        int
	}{
		{"auth.login", "user", adminID, "success", `{"username":"admin@fmcg.com","method":"password"}`, 48},
		{"period.close", "period", periodSepID, "success", `{"period":"2026-09","closed_by":"hq_admin"}`, 168},
		{"transfer.create", "transfer", "90000001-0001-0000-0000-000000000001", "success", `{"amount":20000000,"from":"CASH-JONI","to":"BANK-BCA"}`, 48},
		{"transfer.create", "transfer", "90000002-0002-0000-0000-000000000002", "success", `{"amount":15000000,"from":"BANK-BCA","to":"CASH-001"}`, 72},
		{"invoice.create", "invoice", inv101, "success", `{"code":"INV-2026-0101","customer":"Toko Sumber Rezeki","amount":25000000}`, 24},
		{"invoice.create", "invoice", inv105, "success", `{"code":"INV-2026-0105","customer":"Warung Madura 24 Jam","amount":15000000}`, 12},
		{"payment.record", "payment", inv102, "success", `{"code":"INV-2026-0102","customer":"Warung Berkah Jaya","amount":7000000}`, 72},
		{"reconciler.run", "reconciler", periodOctID, "success", `{"status":"balanced","imbalance":0,"hash_chain_ok":true}`, 1},
	}

	for _, a := range auditEvents {
		eventTime := now.Add(-time.Duration(a.hoursAgo) * time.Hour)
		_, err = conn.Exec(ctx, `
			INSERT INTO audit_logs (id, tenant_id, actor_id, actor_type, action, resource_type, resource_id, outcome, request_id, ip_address, user_agent, metadata, occurred_at)
			VALUES (gen_random_uuid(), $1, $2, 'user', $3, $4, $5, $6, 'req-' || substr(md5(random()::text), 1, 8), '127.0.0.1'::inet, 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)', $7::jsonb, $8)
		`, tenantID, adminID, a.action, a.resType, a.resID, a.outcome, a.meta, eventTime)
		if err != nil {
			log.Fatalf("audit_log %s: %v", a.action, err)
		}
	}
	log.Printf("✓ %d Audit Log entries recorded", len(auditEvents))

	fmt.Println("\n=======================================================")
	fmt.Println("🚀 SEED COMPLETED SUCCESSFULLY!")
	fmt.Println("All menus in Web Dashboard now have interconnected data:")
	fmt.Println("  1. Dashboard: Cards, Stats, Open Invoices, Aging Overview")
	fmt.Println("  2. Accounts: 13 Active Accounts (Cash, Banks, AR, Revenue, Customers)")
	fmt.Println("  3. Transfers: 6 Balanced Double-Entry Transactions with SHA-256 Hash Chain")
	fmt.Println("  4. Invoices: 7 Invoices across all statuses (open, partial, paid, overdue)")
	fmt.Println("  5. Aging: Customer Receivables across 6 aging buckets")
	fmt.Println("  6. Periods: Sept Closed & Oct Open")
	fmt.Println("  7. Reconciler: Balanced runs with 0 imbalance & valid hash chains")
	fmt.Println("  8. Currencies: IDR, USD, SGD with live FX rates")
	fmt.Println("  9. Audit: 8 real audit trails linking actors, actions, and entities")
	fmt.Println("=======================================================")
}
