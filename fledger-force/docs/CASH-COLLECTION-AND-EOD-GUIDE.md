# Panduan Operasional Penerimaan Kas Lapangan & Rekonsiliasi EOD (Anti-Cash Kitting)

Dokumen ini menjelaskan mekanisme operasional pencegahan kecurangan *cash kitting* (gali lubang tutup lubang) pada tenaga salesman keliling melalui integrasi **Fledger Force** dan **Fledger Core**.

---

## 1. Mekanisme Perpindahan Tanggung Jawab Finansial (Liability Shift)

```
[KASUS KONVENSIONAL - RENTAN FRAUD]
Toko Bayar Tunai Rp 20 Jt ──► Uang Masuk Kantong Sales ──► Kantor HQ Tidak Tahu
                                (Dipakai Pribadi 3 Hari)

================================================================================

[MEKANISME FLEDGER FORCE - ZERO FRAUD]
1. Toko Bayar Tunai Rp 20 Jt
         │
         ▼
2. Sales Input di HP: No Faktur & Nominal
         │
         ├──► WhatsApp Otomatis ke Pemilik Toko: "Kwitansi RCP-09812 Terbit, Lunas Rp 20 Jt"
         │
         ▼
3. Fledger Core Mencatat Transaksi Double-Entry Instan:
         Debit  : Account:Salesman-Wallet  Rp 20.000.000 (Hutang Salesman)
         Kredit : Account:Customer-AR      Rp 20.000.000 (Piutang Toko Berkurang)
```

Dengan sistem ini:
1. **Pemilik Toko Terlindungi**: Toko memiliki bukti bayar sah. Salesman tidak bisa mengaku bahwa toko menolak bayar.
2. **Salesman Mengemban Hutang Pribadi**: Di mata akuntansi Fledger Core, uang tersebut bukan lagi piutang toko, melainkan **saldo hutang salesman kepada perusahaan**. Jika uang hilang, salesman wajib mengganti secara hukum.

---

## 2. Alur Tutup Buku Kasir Gudang Sore Hari (End-of-Day Settlement)

Setiap hari kerja pukul 16:30 – 18:00 saat salesman kembali ke kantor gudang:

```
+--------------------------------------------------------------------------------+
|                         MEJA KASIR HQ SETTLEMENT                               |
|                                                                                |
|  Salesman: Budi Santoso (SLS-JKT-004)                                          |
|  Saldo Wajib Setor Sistem : Rp 17.500.000 (dari 5 Toko Hari Ini)              |
|                                                                                |
|  Uang Fisik Dihitung Kasir: [ Rp 17.500.000 ]                                  |
|  Selisih (Discrepancy)    : Rp 0 (PAS)                                         |
|                                                                                |
|  [ ✅ KONFIRMASI SETORAN KAS FISIK EOD & BUKA KUNCI ORDER BESOK ]              |
+--------------------------------------------------------------------------------+
```

### Mutasi Akuntansi di Fledger Core:
Ketika kasir menekan tombol konfirmasi setoran:
$$\text{Debit: Account:HQ-Physical-Cash} \quad \text{Rp 17.500.000}$$
$$\text{Kredit: Account:Salesman-Wallet} \quad \text{Rp 17.500.000}$$

Dampaknya:
1. Saldo wallet salesman kembali menjadi **Rp 0**.
2. Kas fisik di brankas kantor pusat bertambah resmi.
3. Status salesman dibuka kembali menjadi `ACTIVE` sehingga dapat mengunduh rute pesanan toko esok pagi.

---

## 3. Penanganan Selisih Kas (Discrepancy Handling)

Jika terjadi selisih (misalnya salesman hanya membawa uang Rp 17.000.000 padahal sistem mencatat Rp 17.500.000):
1. Kasir memilih status **`DISCREPANCY_FLAGGED`** dengan selisih minus Rp 500.000.
2. Saldo hutang Rp 500.000 tetap menggantung di wallet salesman di Fledger Core.
3. Supervisor menerima notifikasi audit. Status salesman terkunci (`SETTLEMENT_LOCKED`) hingga selisih kas diselesaikan atau disetujui pemotongan gaji via otorisasi Finance.
