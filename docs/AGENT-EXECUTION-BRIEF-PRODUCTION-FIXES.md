# FLEDGER OS — AI Agent Execution Brief: Production Hardening & Bugfix Specification

> **Target Audience**: AI Coding Agent / Autonomous Engineering Agent  
> **Ecosystem**: **FLEDGER OS** (*The Ledger-First Operating System for FMCG Distribution & Logistics*)  
> **Workspace**: `c:\Dev\fledger`  
> **Objective**: Selesaikan 6 isu kritikal fungsional dan 2 optimasi operasional agar ekosistem microservices berjalan **100% Plug-and-Play**, stabil, dan siap untuk deployment live production.  
> **Definition of Done (DoD)**: Seluruh test unit/integrasi pass 100%, `docker compose up -d` pada mesin baru langsung siap pakai tanpa manual SQL, seluruh web portal berfungsi baik via port langsung maupun API Gateway, dan script master E2E `scripts/test-ecosystem-e2e.ps1` lulus 100%.

---

## 📋 Daftar Pekerjaan (Execution Tasks Checklist)

| No | Kategori | Target File / Service | Tingkat Urgensi | Status |
|---|---|---|---|---|
| **1** | Inter-Service Auth | `fledger-order/src/internal/integration/{coreclient,fleetclient}` | 🔴 **CRITICAL** | Ready to Execute |
| **2** | Container Packaging | `fledger-fleet/Dockerfile` | 🔴 **CRITICAL** | Ready to Execute |
| **3** | Database Bootstrapping | `deployments/docker/postgres/init-databases.sh` & `docker-compose.yml` | 🔴 **CRITICAL** | Ready to Execute |
| **4** | Gateway Routing | `deployments/gateway/index.html` & `deployments/gateway/nginx.conf` | 🔴 **CRITICAL** | Ready to Execute |
| **5** | Frontend Sub-Path SDK | `fledger-{order,pay,force,dunning}/**/{*-client.js}` | 🔴 **CRITICAL** | Ready to Execute |
| **6** | Event-Driven Webhook | `fledger-pay/src/internal/{integration/dunningclient,usecase}` | 🟡 **HIGH** | Ready to Execute |
| **7** | Docker Optimization | `docker-compose.yml` (Log Rotation & Memory Limits) | 🟢 **NORMAL** | Ready to Execute |
| **8** | Master FMCG Seeder | `scripts/seed-ecosystem.ps1` | 🟢 **NORMAL** | Ready to Execute |

---

## 🛠️ Rincian Teknis & Panduan Implementasi

---

### Task 1: Pasang Minting Service JWT di `fledger-order` untuk `Core` & `Fleet`

#### 🔍 Masalah
`fledger-core` dan `fledger-fleet` memproteksi semua endpoint mutasi finansial dan penerbitan Surat Jalan dengan middleware `RequireAuth` yang mewajibkan header `Authorization: Bearer <JWT>`.  
Saat ini, `fledger-order` hanya mengirimkan `X-API-Key` dan `X-Tenant-ID`. Akibatnya, saat order dibuat atau disetujui, panggilan ke Core (cek kredit) dan Fleet (terbitkan DO) ditolak dengan **`HTTP 401 Unauthorized`**.

#### 📝 Langkah Perbaikan
1. **Buka file `fledger-order/src/internal/integration/coreclient/client.go`**:
   - Tambahkan field `JWT string` pada `struct Config` dan `struct Client`.
   - Di fungsi `setHeadersWithIdem()`, tambahkan:
     ```go
     if c.jwt != "" {
         req.Header.Set("Authorization", "Bearer "+c.jwt)
     }
     ```
2. **Buka file `fledger-order/src/internal/integration/fleetclient/client.go`**:
   - Tambahkan field `JWT string` pada `struct Config` dan `struct Client`.
   - Di fungsi `CreateDeliveryOrder()`, tambahkan:
     ```go
     if c.jwt != "" {
         req.Header.Set("Authorization", "Bearer "+c.jwt)
     }
     ```
3. **Buka file `fledger-order/src/cmd/api/main.go`**:
   - Tambahkan fungsi pencetak service token:
     ```go
     func mintServiceJWT(secret, tenantID string) string {
         signer := jwt.NewSigner(jwt.StaticSecret{Value: []byte(secret)})
         tok, _ := signer.Sign(jwt.Claims{
             UserID:   "service:fledger-order",
             TenantID: tenantID,
             Role:     "service",
             Scopes:   []string{"*"},
         }, 24*time.Hour)
         return tok
     }
     ```
   - Oper hasil `mintServiceJWT(cfg.JWTSecret, cfg.FledgerTenantID)` ke dalam inisialisasi `coreclient.Config{ JWT: ... }` dan `fleetclient.Config{ JWT: ... }`.

---

### Task 2: Salin Web Portal Statis ke dalam Runtime `fledger-fleet/Dockerfile`

#### 🔍 Masalah
Di `fledger-fleet/src/cmd/api/main.go`, router melayani portal POD supir dengan mencari folder `web/` di sistem berkas lokal. Namun pada `fledger-fleet/Dockerfile`, stage runtime distroless hanya menyalin binary Go `/app/api`, tanpa menyalin folder `web/`. Hal ini menyebabkan endpoint `http://localhost:8082/web` mengembalikan **HTTP 404** di dalam Docker.

#### 📝 Langkah Perbaikan
1. **Buka file `fledger-fleet/Dockerfile`**:
   - Pada stage builder, pastikan context build menyalin `web/`:
     ```dockerfile
     # --- Stage 1: Build ---
     FROM golang:1.23-alpine AS builder
     WORKDIR /app
     COPY src/go.mod src/go.sum ./
     RUN go mod download
     COPY src/ ./src/
     COPY web/ ./web/
     RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
         go build -trimpath \
         -ldflags="-s -w -X main.version=${VERSION}" \
         -o /out/api ./src/cmd/api
     ```
   - Pada stage runtime:
     ```dockerfile
     # --- Stage 2: Runtime ---
     FROM gcr.io/distroless/static-debian12:nonroot
     WORKDIR /app
     COPY --from=builder /out/api /app/api
     COPY --from=builder /app/web /app/web
     ENV WEB_DIR=/app/web
     EXPOSE 8082
     ENTRYPOINT ["/app/api"]
     ```

---

### Task 3: Otomatisasi Eksekusi Skema Database Saat PostgreSQL Cold-Boot

#### 🔍 Masalah
Script `deployments/docker/postgres/init-databases.sh` saat ini hanya membuat database kosong (`CREATE DATABASE ...`). Ketika dijalankan pada komputer atau VPS baru dengan Docker volume bersih, tabel-tabel di dalam ke-6 database tidak terbuat, sehingga microservice langsung crash saat menerima query pertama.

#### 📝 Langkah Perbaikan
1. **Update `docker-compose.yml` pada service `postgres`**:
   Mount seluruh folder migrasi ke direktori `/docker-entrypoint-initdb.d/schemas`:
   ```yaml
   volumes:
     - postgres_data:/var/lib/postgresql/data
     - ./deployments/docker/postgres/init-databases.sh:/docker-entrypoint-initdb.d/01-init-databases.sh:ro
     - ./fledger-core/migrations:/docker-entrypoint-initdb.d/schemas/core:ro
     - ./fledger-fleet/src/migrations:/docker-entrypoint-initdb.d/schemas/fleet:ro
     - ./fledger-pay/src/migrations:/docker-entrypoint-initdb.d/schemas/pay:ro
     - ./fledger-force/src/migrations:/docker-entrypoint-initdb.d/schemas/force:ro
     - ./fledger-order/src/migrations:/docker-entrypoint-initdb.d/schemas/order:ro
     - ./fledger-dunning/docs/DATABASE-SCHEMA.sql:/docker-entrypoint-initdb.d/schemas/dunning/000001_init.sql:ro
   ```
2. **Update `deployments/docker/postgres/init-databases.sh`**:
   Tambahkan eksekusi skrip migrasi `.sql` ke masing-masing database secara berurutan:
   ```bash
   #!/bin/bash
   set -e

   echo ">>> [FLEDGER OS] Creating dedicated microservice databases..."
   DATABASES=("fmcg_wallet" "fledger_fleet" "fledger_pay" "fledger_force" "fledger_order" "fledger_dunning")

   for db in "${DATABASES[@]}"; do
       psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
           SELECT 'CREATE DATABASE $db'
           WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$db')\gexec
           GRANT ALL PRIVILEGES ON DATABASE $db TO $POSTGRES_USER;
   EOSQL
   done

   echo ">>> [FLEDGER OS] Applying initial database migrations..."
   # 1. Fledger Core (semua up.sql berurutan)
   for f in $(ls /docker-entrypoint-initdb.d/schemas/core/*up.sql | sort); do
       echo "  -> Core migration: $f"
       psql -v ON_ERROR_STOP=0 --username "$POSTGRES_USER" --dbname "fmcg_wallet" -f "$f" || true
   done

   # 2. Fledger Fleet
   for f in $(ls /docker-entrypoint-initdb.d/schemas/fleet/*.sql | sort); do
       echo "  -> Fleet migration: $f"
       psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "fledger_fleet" -f "$f"
   done

   # 3. Fledger Pay
   psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "fledger_pay" -f /docker-entrypoint-initdb.d/schemas/pay/000001_init_pay.sql

   # 4. Fledger Force
   psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "fledger_force" -f /docker-entrypoint-initdb.d/schemas/force/000001_init_force.sql

   # 5. Fledger Order
   psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "fledger_order" -f /docker-entrypoint-initdb.d/schemas/order/000001_init_order.sql

   # 6. Fledger Dunning
   psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "fledger_dunning" -f /docker-entrypoint-initdb.d/schemas/dunning/000001_init.sql

   echo ">>> [FLEDGER OS] All microservice databases and schemas are initialized successfully!"
   ```

---

### Task 4: Perbaiki Tautan Port Statis di API Gateway (`index.html`)

#### 🔍 Masalah
Di `deployments/gateway/index.html`, seluruh tombol mengarah ke `http://localhost:808X/web`. Jika diakses melalui server VPS publik atau LAN, browser klien mencari `localhost` di perangkat mereka sendiri dan koneksi terputus (*connection refused*).

#### 📝 Langkah Perbaikan
1. **Buka file `deployments/gateway/index.html`**:
   Ganti seluruh `href="http://localhost:808X/web"` menjadi path relatif reverse proxy Gateway:
   - Fledger Core: `<a href="/core/" class="btn">Buka Core Ledger</a>`
   - Fledger Fleet: `<a href="/fleet/web/" class="btn">Buka Fleet Portal</a>`
   - Fledger Pay: `<a href="/pay/" class="btn">Buka Payment Sandbox</a>`
   - Fledger Force: `<a href="/force/" class="btn">Buka SFA Mobile PWA</a>`
   - Fledger Order: `<a href="/order/" class="btn">Buka OMS Order Portal</a>`
   - Fledger Dunning: `<a href="/dunning/" class="btn">Buka Dunning Portal</a>`

---

### Task 5: Optimasi Web Client SDK agar Mendukung Sub-Path API Gateway

#### 🔍 Masalah
Ketika web portal dibuka melalui Gateway (misal: `http://localhost:80/order/`), variabel `location.origin` bernilai `http://localhost:80`. Ketika client JS memanggil `fetch(baseURL + '/v1/order/orders')`, request menjadi `http://localhost:80/v1/order/orders` (kehilangan prefix `/order/`), sehingga terkena aturan root Nginx dan menghasilkan **HTTP 404**.

#### 📝 Langkah Perbaikan
Terapkan deteksi sub-path dinamis pada seluruh client JavaScript:
1. **`fledger-order/web/order-client.js`** & file embedded-nya:
   ```javascript
   const subpath = location.pathname.startsWith('/order') ? '/order' : '';
   const baseURL = (location.origin && location.origin !== 'null') ? (location.origin + subpath) : 'http://localhost:8085';
   ```
2. **`fledger-pay/web/pay-client.js`** & file embedded-nya:
   ```javascript
   const subpath = location.pathname.startsWith('/pay') ? '/pay' : '';
   const baseURL = (location.origin && location.origin !== 'null') ? (location.origin + subpath) : 'http://localhost:8083';
   ```
3. **`fledger-force/web/force-client.js`** & file embedded-nya:
   ```javascript
   const subpath = location.pathname.startsWith('/force') ? '/force' : '';
   const baseURL = (location.origin && location.origin !== 'null') ? (location.origin + subpath) : 'http://localhost:8084';
   ```
4. **`fledger-dunning/internal/webui/files/dunning-client.js`**:
   ```javascript
   const subpath = location.pathname.startsWith('/dunning') ? '/dunning' : '';
   const baseURL = (location.origin && location.origin !== 'null') ? (location.origin + subpath) : 'http://localhost:8086';
   ```

---

### Task 6: Sambungkan Webhook Pelunasan `fledger-pay` ke `fledger-dunning`

#### 🔍 Masalah
`docker-compose.yml` telah menyediakan `FLEDGER_DUNNING_URL: http://dunning:8086` pada `pay`, dan `fledger-dunning` memiliki endpoint `POST /v1/dunning/webhooks/pay`. Namun di kode `fledger-pay`, `SettlementService` belum memiliki HTTP client untuk memicu webhook pembatalan dunning secara otomatis saat pembayaran lunas.

#### 📝 Langkah Perbaikan
1. **Buat file `fledger-pay/src/internal/integration/dunningclient/client.go`**:
   ```go
   package dunningclient

   import (
       "bytes"
       "context"
       "encoding/json"
       "fmt"
       "net/http"
       "time"
   )

   type Client struct {
       baseURL       string
       webhookSecret string
       httpClient    *http.Client
   }

   func NewClient(baseURL, webhookSecret string) *Client {
       return &Client{
           baseURL:       baseURL,
           webhookSecret: webhookSecret,
           httpClient:    &http.Client{Timeout: 5 * time.Second},
       }
   }

   type PaymentSettledPayload struct {
       Event           string `json:"event"`
       InvoiceID       string `json:"invoice_id"`
       PaidAmountMinor int64  `json:"paid_amount_minor"`
       PaidAt          string `json:"paid_at"`
       SettlementRef   string `json:"settlement_ref"`
   }

   func (c *Client) NotifyPaymentSettled(ctx context.Context, tenantID, invoiceID string, amount int64, ref string) error {
       if c.baseURL == "" || invoiceID == "" {
           return nil
       }
       payload := PaymentSettledPayload{
           Event:           "payment.settled",
           InvoiceID:       invoiceID,
           PaidAmountMinor: amount,
           PaidAt:          time.Now().UTC().Format(time.RFC3339),
           SettlementRef:   ref,
       }
       b, _ := json.Marshal(payload)
       req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/dunning/webhooks/pay", bytes.NewReader(b))
       if err != nil {
           return err
       }
       req.Header.Set("Content-Type", "application/json")
       req.Header.Set("X-Tenant-ID", tenantID)
       req.Header.Set("X-Webhook-Secret", c.webhookSecret)

       resp, err := c.httpClient.Do(req)
       if err != nil {
           return err
       }
       defer resp.Body.Close()
       if resp.StatusCode >= 400 {
           return fmt.Errorf("dunning webhook returned status %d", resp.StatusCode)
       }
       return nil
   }
   ```
2. **Pasang client ke dalam `fledger-pay/src/internal/usecase/settlement_service.go`**:
   - Di `OutboxDrainOnce`, setelah sukses memanggil `core.MarkInvoicePaid`:
     ```go
     if s.dunning != nil && invoiceID != "" {
         _ = s.dunning.NotifyPaymentSettled(ctx, e.TenantID, invoiceID, e.GrossAmount, e.TransactionID)
     }
     ```
3. **Wiring di `fledger-pay/src/cmd/api/main.go`**:
   - Baca `FLEDGER_DUNNING_URL` dan `WEBHOOK_SECRET` dari config/env, inisialisasi `dunningclient.NewClient(...)`, dan pasang ke `settlementService`.

---

### Task 7: Optimasi Resource Limits & Log Rotation di `docker-compose.yml`

#### 🔍 Masalah
Tanpa batas memori dan rotasi log, penggunaan Docker di VPS atau dev laptop berisiko mengalami *OOM Crash* dan kehabisan memori hard disk akibat log yang membengkak terus menerus.

#### 📝 Langkah Perbaikan
1. Di bagian atas `docker-compose.yml`, tambahkan YAML extension block:
   ```yaml
   x-logging: &default-logging
     driver: "json-file"
     options:
       max-size: "10m"
       max-file: "3"
   ```
2. Pasang `logging: *default-logging` pada seluruh service (`postgres`, `redis`, `nats`, `core`, `fleet`, `pay`, `force`, `order`, `dunning`, `gateway`).
3. Tambahkan batas RAM yang aman:
   - `postgres`: `mem_limit: 1024m`
   - `redis`: `mem_limit: 256m`
   - `nats`: `mem_limit: 256m`
   - Microservices Go (`core`, `fleet`, `pay`, `force`, `order`, `dunning`): `mem_limit: 512m` masing-masing
   - `gateway`: `mem_limit: 128m`

---

### Task 8: Buat Script Master FMCG Demo Seeder (`scripts/seed-ecosystem.ps1`)

#### 🔍 Masalah
Setelah container baru menyala, database masih kosong dari data demo operasional FMCG (toko retailer, armada, produk, piutang macet), sehingga presentasi live walkthrough harus input manual satu per satu.

#### 📝 Langkah Perbaikan
Buat skrip `scripts/seed-ecosystem.ps1` yang otomatis mengisi:
1. **1 Tenant Distributor Utama**: `PT Fledger Distribusi Nusantara` (`a0000000-0000-0000-0000-000000000001`).
2. **Chart of Accounts di Core**: Kas Gudang HQ, Bank BCA Operasional, Kas Salesman Budi, Kas Salesman Andi, Piutang Toko.
3. **2 Truk & 2 Supir di Fleet**: `B 9102 FMC` (Truk Engkel Box) & `B 9204 FMC` (Blindvan GranMax), Supir Pak Joko & Pak Dani.
4. **5 Toko Retailer di Force & Order**: Toko Berkah Mandiri, Toko Sumber Rezeki, Toko Sinar Jaya, Toko Rezeki Abadi, Toko Makmur. Lengkap dengan koordinat GPS Jakarta.
5. **Katalog Produk FMCG Populer di Order**:
   - `SKU-INDOMIE-GRG` (Indomie Goreng Rasa Ayam Panggang) — Rp 115.000 / dus.
   - `SKU-MINYAK-2L` (Minyak Goreng Sawit 2L) — Rp 34.000 / pouch.
   - `SKU-AQUA-600ML` (Air Mineral Aqua 600ml x 24) — Rp 48.000 / karton.
   - `SKU-KOPI-KAPALAPI` (Kopi Bubuk Special 165g) — Rp 16.500 / pack.
6. **2 Faktur Berjalan di Dunning**: 1 faktur H-3 jatuh tempo dan 1 faktur menunggak H+7 untuk simulasi live alert.

---

## 🧪 Validasi Akhir & Verification Commands

Setelah seluruh 8 task di atas diselesaikan oleh AI Agent, jalankan langkah verifikasi berurutan:

```powershell
# 1. Jalankan unit test di setiap modul Go
cd fledger-order && go test ./... && cd ..
cd fledger-fleet/src && go test ./... && cd ../..
cd fledger-pay/src && go test ./... && cd ../..
cd fledger-force/src && go test ./... && cd ../..
cd fledger-dunning && go test ./... && cd ..

# 2. Rebuild & nyalakan seluruh ekosistem Docker
docker compose down -v
docker compose build
docker compose up -d

# 3. Periksa status seluruh container
docker compose ps

# 4. Jalankan Master E2E Ecosystem Test
powershell -ExecutionPolicy Bypass -File .\scripts\test-ecosystem-e2e.ps1

# 5. Jalankan Master Seeder
powershell -ExecutionPolicy Bypass -File .\scripts\seed-ecosystem.ps1

# 6. Buka Gateway Portal di Browser
# http://localhost/
```

**Kriteria Lolos (Definition of Done)**:
- Output `test-ecosystem-e2e.ps1` menghasilkan status:  
  `[OK] PASSED: THE GOLDEN FMCG FLOW IS VERIFIED ACROSS ALL SERVICES!`
- Seluruh kartu modul pada Gateway `http://localhost/` dapat dibuka dan tidak menghasilkan error 401/404 pada API fetch-nya.
