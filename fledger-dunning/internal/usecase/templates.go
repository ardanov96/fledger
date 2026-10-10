// Package usecase ??? spintax + message templates for the 5-stage cadence.
package usecase

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/fledger/fledger-dunning/internal/domain"
)

// GreetingOptions for the PRE_DUE_H3 reminder (anti-ban variation).
var greetings = []string{
	"Selamat pagi",
	"Selamat siang",
	"Selamat sore",
	"Yth. Bapak/Ibu",
	"Salam hormat",
	"Halo",
}

// paymentMessage returns the WhatsApp body for a given stage, including
// the deep-link to Fledger Pay.
func paymentMessage(stage domain.DunningStage, storeName, invoiceNumber string, amountMinor int64, dueDate time.Time, paymentLink string) string {
	rupiah := formatIDR(amountMinor)
	dueStr := dueDate.Format("02 Jan 2006")
	switch stage {
	case domain.StagePreDueH3:
		sal := randomGreeting()
		return fmt.Sprintf(
			"???? *PENGINGAT JATUH TEMPO FAKTUR*\n\n"+
				"%s Pemilik *%s*,\n"+
				"Tagihan Faktur *%s* senilai *%s* akan jatuh tempo dalam *3 hari* pada tanggal *%s*.\n\n"+
				"Untuk kenyamanan transaksi Anda, pembayaran dapat dilakukan secara instan via QRIS / BCA Virtual Account melalui tautan resmi:\n"+
				"???? %s\n\n"+
				"_Abaikan pesan ini jika Anda telah melakukan pembayaran._",
			sal, storeName, invoiceNumber, rupiah, dueStr, paymentLink,
		)
	case domain.StageDueDate:
		sal := randomGreeting()
		return fmt.Sprintf(
			"?????? *HARI JATUH TEMPO FAKTUR*\n\n"+
				"%s Pemilik *%s*,\n"+
				"Faktur *%s* senilai *%s* jatuh tempo *HARI INI*.\n\n"+
				"Mohon segera selesaikan pembayaran untuk menjaga kelancaran pengiriman pesanan berikutnya:\n"+
				"???? %s",
			sal, storeName, invoiceNumber, rupiah, paymentLink,
		)
	case domain.StageOverdueH3:
		sal := randomGreeting()
		return fmt.Sprintf(
			"??? *PEMBERITAHUAN KETERLAMBATAN PEMBAYARAN*\n\n"+
				"%s Bapak/Ibu *%s*,\n"+
				"Pembayaran Faktur *%s* senilai *%s* telah terlambat *3 hari*.\n\n"+
				"Mohon segera lakukan pembayaran hari ini via tautan berikut:\n"+
				"???? %s",
			sal, storeName, invoiceNumber, rupiah, paymentLink,
		)
	case domain.StageOverdueH7:
		sal := randomGreeting()
		return fmt.Sprintf(
			"???? *PERINGATAN TUNGGAKAN PIUTANG (H+7)*\n\n"+
				"%s Bapak/Ibu *%s*,\n"+
				"Faktur *%s* senilai *%s* belum kami terima dan telah melewati batas toleransi 7 hari.\n\n"+
				"Harap segera menyelesaikan pelunasan sebelum fasilitas kredit toko ditangguhkan.\n"+
				"???? %s",
			sal, storeName, invoiceNumber, rupiah, paymentLink,
		)
	case domain.StageOverdueH14:
		sal := randomGreeting()
		return fmt.Sprintf(
			"???? *PEMBERITAHUAN PENANGGUHAN PESANAN (CREDIT BLOCKED)*\n\n"+
				"%s Manajemen *%s*,\n"+
				"Karena tunggakan faktur telah melampaui 14 hari, sistem *Fledger Order* secara otomatis telah *MEMBEKUKAN (CREDIT_BLOCKED)* penerbitan Surat Jalan baru untuk toko Anda.\n\n"+
				"Fasilitas pesanan akan otomatis aktif kembali segera setelah pelunasan terkonfirmasi:\n"+
				"???? %s",
			sal, storeName, paymentLink,
		)
	}
	return fmt.Sprintf("Tagihan %s: %s. Bayar di %s", invoiceNumber, rupiah, paymentLink)
}

func paidConfirmation(storeName, invoiceNumber string, amountMinor int64) string {
	return fmt.Sprintf(
		"??? *PEMBAYARAN DITERIMA*\n\n"+
			"Terima kasih Bapak/Ibu *%s*.\n"+
			"Pembayaran Faktur *%s* senilai *%s* telah kami terima.\n\n"+
			"Fasilitas pemesanan Anda di Fledger Order tetap aktif normal. "+
			"Selamat berbisnis kembali! ????",
		storeName, invoiceNumber, formatIDR(amountMinor),
	)
}

// randomGreeting picks one of the salutations; uses math/rand seeded by
// the runtime which is fine for our jitter purposes.
func randomGreeting() string {
	return greetings[rand.Intn(len(greetings))]
}

func formatIDR(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	digits := fmt.Sprintf("%d", n)
	out := []byte{}
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(c))
	}
	prefix := "Rp "
	if neg {
		prefix = "-Rp "
	}
	return prefix + string(out)
}

// StageLabel returns a human-readable Indonesian label.
func StageLabel(stage domain.DunningStage) string {
	switch stage {
	case domain.StagePreDueH3:
		return "PRE_DUE_H3"
	case domain.StageDueDate:
		return "DUE_DATE"
	case domain.StageOverdueH3:
		return "OVERDUE_H3"
	case domain.StageOverdueH7:
		return "OVERDUE_H7"
	case domain.StageOverdueH14:
		return "OVERDUE_H14"
	}
	return strings.ToUpper(string(stage))
}