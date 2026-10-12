# Desain Landing Page SaaS E-Wallet Logistik FMCG dengan Launch UI

Oct 12, 2026 · @Ardha

## Ringkasan

Launch UI adalah fondasi yang paling pas untuk landing page ini, karena tampilannya bersih dan serius, cocok dengan citra produk finansial B2B. Ini template landing SaaS open source berbasis shadcn dan Next.js, dengan struktur section lengkap: hero, fitur, pricing, FAQ, dan CTA.

Audiens produk ini adalah distributor, tim finance, dan owner toko FMCG. Bagi mereka, rasa percaya lebih penting daripada gaya eksperimental, jadi identitas yang dituju adalah **tepercaya, rapi, dan berbasis data**.

Alasan Launch UI cocok:

- Berbasis shadcn/ui, sehingga warna, font, dan radius bisa diatur lewat design token di satu tempat.
- Pola kodenya dikenal baik oleh AI, jadi nyaman dipakai untuk vibe coding.
- Struktur section sudah ada, tinggal mengganti konten dengan cerita produk e-wallet logistik FMCG.

## Identitas visual

Tetapkan empat keputusan visual ini sebelum membuat section apa pun, lalu simpan sebagai variabel tema shadcn (misalnya `--primary` dan `--radius`) supaya semua section konsisten.

| Elemen | Arahan | Alasan |
| --- | --- | --- |
| Warna | Biru tua atau teal-hijau sebagai warna utama, abu netral untuk latar, satu warna aksen untuk CTA | Biru dan hijau memberi kesan stabil dan uang; satu aksen menjaga fokus ke CTA |
| Font | Inter atau Geist untuk teks; angka memakai gaya tabular | Angka rapi sejajar di tabel dan nominal |
| Bentuk | Radius kecil sampai sedang, bayangan tipis, banyak ruang kosong | Terasa serius dan seperti produk finansial |
| Visual utama | Mockup dashboard asli (saldo, aging piutang, riwayat net-off) | Calon pengguna melihat produknya, bukan ilustrasi 3D atau foto stock |

Panel preview di situs Launch UI menyediakan beberapa tema warna dan empat pilihan radius sudut (small, default, large, xl). Dari namanya, tema Titanium (netral), Ice (biru), dan Emerald (hijau) paling dekat dengan arahan di atas, dan radius small atau default paling sesuai. Lihat tampilan aslinya sebelum memilih.

## Pemetaan section ke blok Launch UI

Tujuh section di bawah menutup alur dari janji sampai ajakan bertindak, dan enam di antaranya punya padanan blok langsung di Launch UI. Hanya alur dana yang perlu dibuat khusus.

| # | Section | Isi untuk produk ini | Blok Launch UI |
| --- | --- | --- | --- |
| 1 | Hero | Janji utama dan mockup dashboard | Navbar + Hero (badge, judul, deskripsi, tombol CTA, mockup) |
| 2 | Masalah | Piutang menggantung, rekonsiliasi manual, kredit toko sulit dipantau | Items atau Feature (grid ikon dan deskripsi) |
| 3 | Fitur | Wallet, aging piutang, net-off multi-flow | Bento Grid |
| 4 | Alur dana | Aliran dana antara distributor, sales, dan toko | Tidak ada blok bawaan; buat diagram kustom (lihat bagian pelengkap) |
| 5 | Keamanan dan audit trail | Kontrol akses, jejak transaksi, rekonsiliasi | Feature atau Items |
| 6 | Pricing atau hubungi kami | Paket, atau ajakan demo untuk skema B2B | Pricing atau CTA |
| 7 | FAQ dan penutup | Pertanyaan umum, ajakan terakhir, footer | FAQ + CTA + Footer |

Blok Stats dan Social Proof hanya dipakai kalau sudah ada angka atau testimoni nyata. Jangan isi dengan data contoh.

Launch UI juga punya template penuh. Saturn untuk SaaS umum, dan Jupiter untuk produk analitik yang padat data (metrik dan grafik). Jupiter paling dekat dengan kebutuhan dashboard keuangan. Periksa di halaman Templates apakah template ini masuk versi Free atau Pro sebelum menjadikannya dasar.

## Pelengkap dan hal yang dihindari

Versi Free Launch UI hanya memuat 1 template dan 9 blok, sedangkan versi Pro (US$99 sekali bayar) memuat 74 blok. Kekurangan blok bisa ditutup dengan versi Pro atau dengan pelengkap berikut.

| Pelengkap | Dipakai untuk | Batasan |
| --- | --- | --- |
| HyperUI | Section yang tidak ada di Launch UI, seperti tabel perbandingan dan statistik | Pilih varian sederhana |
| 21st.dev | Pricing, stats, dan logo cloud bergaya minimalis | Cek lisensi tiap komponen |
| Magic UI | Animated Beam untuk alur dana, Number Ticker untuk angka, Bento Grid | Cukup satu atau dua animasi; lebih dari itu terasa seperti startup konsumer |

Launch UI punya varian Hero dan CTA dengan efek glow. Untuk audiens finance, pilih varian tanpa glow atau redam efeknya, karena glow lebih identik dengan produk AI.

Sebaiknya dihindari:

- **Aceternity UI** (spotlight, kartu 3D) dan **Uiverse.io**: terlalu playful untuk audiens finance.
- **Template Bootstrap atau HTML5 UP**: tampilannya generik dan terasa lama.
- **Dribbble dan Behance**: visualnya bagus tetapi sering tidak realistis dibangun, dan jarang cocok untuk B2B.

## Galeri referensi

Gunakan galeri untuk mencari pola dan struktur, sedangkan Launch UI memberi kodenya. Di **Land-book**, **SaaSpo**, dan **Refero**, filter ke kategori fintech atau B2B payments.

Perhatikan landing page Stripe, Mercury, Ramp, Brex, Xendit, dan Midtrans. Pola yang layak ditiru:

- Hero dengan mockup dashboard, bukan ilustrasi.
- Urutan "masalah, lalu solusi" sebelum daftar fitur.
- Blok keamanan yang terlihat jelas, bukan disembunyikan di footer.

Ambil pola layoutnya saja, bukan aset, teks, atau ilustrasinya.

## Catatan regulasi untuk copy

Jangan memakai klaim "dompet digital berlisensi" kecuali produk ini memang penerbit uang elektronik berlisensi. Di Indonesia, istilah "e-wallet" dan "e-money" terkait dengan regulasi Bank Indonesia.

Kalau produknya berupa ledger internal antar mitra bisnis, istilah yang lebih aman dan lebih akurat:

- "Wallet ledger"
- "Manajemen saldo dan piutang"
- "Rekonsiliasi otomatis"

Ini catatan umum, bukan nasihat hukum. Pastikan status produk dan teks klaim di landing page dikonfirmasi ke penasihat hukum atau konsultan kepatuhan sebelum rilis.

## Langkah implementasi dan prompt hero

Urutan kerja terbaik adalah memilih tampilan dulu, baru menyusun section dan mengisi konten.

1. Di situs Launch UI, coba panel preview untuk memilih tema warna dan radius sudut, lalu catat pilihannya.
2. Pasang komponen lewat shadcn CLI mengikuti panduan Getting started di situs.
3. Atur warna, tipografi, dan radius lewat variabel CSS di `globals.css`.
4. Susun section sesuai tabel pemetaan di atas.
5. Isi konten. Data di Launch UI dipisahkan dari komponen, jadi teks dan angka bisa diganti tanpa mengubah kode komponen.
6. Periksa semua klaim di copy dengan catatan regulasi sebelum rilis.

Contoh prompt untuk men-generate hero section dengan AI:

```text
Buat hero section landing page SaaS wallet ledger untuk distribusi FMCG,
memakai blok Hero dari Launch UI (React, shadcn/ui, Tailwind).
Audiens: distributor, tim finance, dan owner toko.
Gaya: tepercaya, rapi, berbasis data. Warna utama biru tua dengan satu
warna aksen untuk tombol CTA. Font Inter, radius sudut kecil, tanpa efek glow.
Visual utama: mockup dashboard berisi saldo, aging piutang, dan riwayat net-off.
Judul menekankan kontrol piutang dan rekonsiliasi otomatis.
Jangan memakai klaim "dompet digital berlisensi".
Pisahkan teks dan data dari komponen.
```

## Sumber

Diakses 12 Oktober 2026.

- [Launch UI](https://www.launchuicomponents.com/): daftar blok dan template, opsi tema dan radius, paket Free dan Pro, serta lisensi.
