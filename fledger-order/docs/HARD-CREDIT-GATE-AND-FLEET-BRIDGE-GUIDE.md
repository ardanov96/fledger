# Panduan Operasional Hard Credit Gate & Dispatching Bridge Fledger Order

Dokumen ini menjelaskan algoritma pencegahan piutang macet (*Hard Credit Gate*) dan jembatan dispatching otomatis ke **Fledger Fleet**.

---

## 1. Masalah Lapangan: Kebutaan Limit Kredit (*Credit Limit Blindness*)

Pada distributor FMCG konvensional:
1. Tim sales dan tim gudang mendapatkan insentif berdasarkan **omset penjualan (volume penjualan)**.
2. Mereka tidak memiliki insentif untuk menahan pesanan dari toko yang menunggak pembayaran.
3. Akibatnya, toko yang sudah menunggak faktur selama 60–90 hari tetap dikirimi barang ratusan karton. Ketika toko tersebut bangkrut atau kabur, distributor menanggung kerugian piutang tak tertagih (*Bad Debt*) hingga ratusan juta rupiah.

---

## 2. Algoritma Dua Langkah: Hard Credit Gate

Sebelum pesanan diubah menjadi status `APPROVED` di **Fledger Order**, sistem mengeksekusi dua aturan evaluasi ketat (*Pre-Flight Check*) ke **Fledger Core**:

```
                       [EVALUASI ORDER BARU Rp 4 Juta]
                                     │
                    ┌────────────────┴────────────────┐
                    ▼                                 ▼
           [ATURAN 1: PLAFON]                [ATURAN 2: OVERDUE]
   (Total AR + Rp 4Jt) <= Limit Toko?    Ada Faktur Menunggak >30 Hari?
                    │                                 │
            ┌───────┴───────┐                 ┌───────┴───────┐
           YA              TIDAK             TIDAK            YA
            │                │                │                │
            ▼                ▼                ▼                ▼
         [LOLOS]     [LIMIT_EXCEEDED]      [LOLOS]    [OVERDUE_BLOCKED]
                    (Kredit Ditolak)                  (Kredit Ditolak)
                    
================================================================================
KONDISI KELULUSAN:
Wajib lolos KEDUA ATURAN di atas sekaligus untuk menjadi: [CREDIT_GATE: PASSED]
Jika salah satu gagal: Status Order terkunci otomatis menjadi: [CREDIT_BLOCKED]
================================================================================
```

### Detail Rumus:
1. **Aturan 1 (Plafon Kredit)**:
   $$\text{Proyeksi AR} = \text{Total Piutang Berjalan} + \text{Nilai Order Baru}$$
   $$\text{Syarat: } \text{Proyeksi AR} \le \text{Credit Limit Toko}$$
2. **Aturan 2 (Toleransi Hari Keterlambatan)**:
   Sistem membaca modul Aging di Fledger Core (`/v1/aging`). Jika toko memiliki saldo terbuka pada bucket:
   - Bucket 31–60 Hari, atau
   - Bucket 61–90 Hari, atau
   - Bucket >90 Hari
   Maka pesanan **LANGSUNG DIBLOKIR**, meskipun plafon rupiahnya masih mencukupi.

---

## 3. Protokol Otorisasi Darurat (Managerial Override)

Jika ada kondisi khusus (misalnya pemilik toko adalah kerabat owner atau berjanji transfer siang hari ini), pesanan yang diblokir dapat dibuka secara resmi hanya oleh **Finance Manager / Direktur**:
1. Menggunakan endpoint `POST /v1/order/orders/:id/override-credit`.
2. Wajib menyertakan PIN otorisasi dan alasan tertulis.
3. Seluruh detail override dicatat permanen di `order_audit_logs` agar dapat dipertanggungjawabkan saat audit tahunan.

---

## 4. Jembatan Otomatis ke Fledger Fleet (Dispatching Bridge)

Begitu status pesanan menjadi `APPROVED`:
1. Sistem menghitung akumulasi total berat barang dalam kilogram:
   $$\text{Total Berat (Kg)} = \sum (\text{Qty} \times \text{Berat Gram}) / 1000$$
2. Event `order.dispatched_fleet` dimasukkan ke tabel `order_outbox`.
3. Background outbox worker memanggil REST API `fledger-fleet` (`:8082`):
   `POST /v1/fleet/delivery-orders`
4. Di dashboard `fledger-fleet`, Surat Jalan (DO) baru langsung muncul dan siap dimasukkan ke dalam rute truk pengantaran hari ini.
