# Panduan Simulator Pembayaran & Auto-Settlement Fledger Pay

Dokumen ini menjelaskan cara kerja **Payment Simulator** dan alur rekonsiliasi otomatis (*Auto-Settlement*) antara **Fledger Pay**, gateway pembayaran, dan **Fledger Core**.

---

## 1. Konsep Simulator Sandbox (Zero-Cost Bank Simulator)

Untuk keperluan pengujian end-to-end, presentasi portfolio, dan integrasi tanpa rekening bank riil, **Fledger Pay** dilengkapi dengan simulator perbankan bawaan yang mensimulasikan alur pembayaran:
1. **Virtual Account Simulator** (BCA, Mandiri, BRI).
2. **QRIS Simulator** (Simulasi scan QR oleh aplikasi perbankan m-banking / e-wallet).

```
+--------------------------------------------------------------------------------+
|                        FLEDGER PAY SANDBOX SIMULATOR                           |
|                                                                                |
|  [Faktur: INV-202610-0089]                                                    |
|  Customer: Toko Sumber Rezeki                                                  |
|  Total Tagihan: Rp 4.000.000                                                   |
|                                                                                |
|  Metode Pembayaran:                                                            |
|  (•) BCA Virtual Account : 89012081298765432       [ Salin No VA ]            |
|  ( ) Mandiri VA          : 88701081298765432                                   |
|  ( ) QRIS Dinamis        : [ Render QR Code ]                                  |
|                                                                                |
|  ----------------------------------------------------------------------------  |
|  Tindakan Simulator (Testing):                                                 |
|  [ ⚡ Simulasikan Pembayaran Berhasil via BCA Mobile (One-Click Settle) ]       |
+--------------------------------------------------------------------------------+
```

Ketika tombol simulator ditekan, sistem:
1. Memanggil endpoint sandbox `POST /v1/pay/simulator/settle`.
2. Menghasilkan event pelunasan resmi sama persis seperti callback webhook bank asli.
3. Mencatat transaksi pelunasan di database.
4. Memicu worker outbox untuk mengirimkan jurnal mutasi ke **Fledger Core**.

---

## 2. Alur Rekonsiliasi Otomatis ke Fledger Core

Setelah status pembayaran menjadi `SETTLED`, sistem mengeksekusi dua langkah di Fledger Core:

### Langkah A: Pembukuan Jurnal Finansial (Double-Entry Ledger)
Fledger Pay memanggil endpoint transfer Fledger Core:
```http
POST /v1/transfers HTTP/1.1
Host: localhost:8081
Authorization: Bearer <FLEDGER_CORE_JWT>
X-Tenant-ID: <TENANT_UUID>
Idempotency-Key: <PAY_TRANSACTION_ID>
Content-Type: application/json

{
  "source_account_id": "ACC_CUSTOMER_AR",
  "destination_account_id": "ACC_BANK_BCA",
  "amount": 4000000,
  "currency": "IDR",
  "description": "Pelunasan otomatis faktur via BCA Virtual Account (PAY-202610-00891)"
}
```

Dampak pada buku besar Fledger Core:
- **Kas Bank BCA** bertambah (Debit: Rp 4.000.000)
- **Piutang Dagang (AR Toko)** berkurang (Kredit: Rp 4.000.000)
- Keseimbangan akuntansi: $\sum \text{Debit} - \sum \text{Kredit} = 0$.

### Langkah B: Perubahan Status Faktur di Fledger Core
Fledger Pay memanggil penanda lunas faktur:
```http
POST /v1/invoices/<INVOICE_UUID>/pay HTTP/1.1
Host: localhost:8081
```
Status invoice di dashboard Fledger Core berubah seketika dari `ISSUED` ➔ `PAID`.

---

## 3. Integrasi dengan Gateway Nyata (Production Mode)

Dalam mode produksi, cukup arahkan URL webhook di dashboard gateway (Midtrans / Xendit / DOKU / Bank Direct):

### Midtrans
- **Webhook URL**: `https://api.fledger.io/v1/pay/webhooks/midtrans`
- **Verifikasi**: Signature dihitung dari `SHA512(order_id + status_code + gross_amount + ServerKey)`

### Xendit
- **Webhook URL**: `https://api.fledger.io/v1/pay/webhooks/xendit`
- **Verifikasi**: Header `x-callback-token` dicocokkan dengan `WEBHOOK_SECRET`.
