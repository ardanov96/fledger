// Package pdf renders the monthly e-statement (rekening koran) PDF using
// jung-kurt/gofpdf. The layout is intentionally simple but elegant ??? header
// block, store info, mutation table, and a closing-balance footer.
package pdf

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jung-kurt/gofpdf"

	"github.com/fledger/fledger-dunning/internal/domain"
)

// OutputDir is the local directory where rendered PDFs are stored. The
// directory is created on first use.
const OutputDir = "./storage/statements"

// Render produces a PDF file for the given statement and writes it to
// OutputDir. The returned path is what gets persisted in
// dunning_statements.pdf_file_path.
func Render(s domain.Statement, storeName, ownerName, storePhone string, lines []domain.StatementLineItem) (string, error) {
	if err := os.MkdirAll(OutputDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", OutputDir, err)
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	drawHeader(pdf, s, storeName, ownerName, storePhone)
	drawTable(pdf, lines)
	drawFooter(pdf, s)

	fileName := fmt.Sprintf("%s-%s.pdf", s.StatementMonth, s.StoreID)
	path := filepath.Join(OutputDir, fileName)
	if err := pdf.OutputFileAndClose(path); err != nil {
		return "", fmt.Errorf("write pdf %s: %w", path, err)
	}
	return path, nil
}

func drawHeader(pdf *gofpdf.Fpdf, s domain.Statement, storeName, ownerName, storePhone string) {
	pdf.SetFont("Helvetica", "B", 18)
	pdf.Cell(0, 10, "FLEDGER DISTRIBUTOR")
	pdf.Ln(8)
	pdf.SetFont("Helvetica", "", 10)
	pdf.Cell(0, 5, "Rekening Koran Toko - Monthly Account Statement")
	pdf.Ln(8)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.Cell(0, 6, fmt.Sprintf("Periode: %s", s.StatementMonth))
	pdf.Ln(8)

	pdf.SetFont("Helvetica", "B", 10)
	pdf.Cell(40, 6, "Toko:")
	pdf.SetFont("Helvetica", "", 10)
	pdf.Cell(0, 6, storeName)
	pdf.Ln(5)
	pdf.Cell(40, 6, "Store ID:")
	pdf.Cell(0, 6, s.StoreID)
	pdf.Ln(5)
	pdf.Cell(40, 6, "Pemilik:")
	pdf.Cell(0, 6, ownerName)
	pdf.Ln(5)
	pdf.Cell(40, 6, "No HP:")
	pdf.Cell(0, 6, storePhone)
	pdf.Ln(10)
}

func drawTable(pdf *gofpdf.Fpdf, lines []domain.StatementLineItem) {
	// Header row.
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(230, 230, 230)
	pdf.CellFormat(20, 7, "Tanggal", "1", 0, "C", true, 0, "")
	pdf.CellFormat(20, 7, "Ref", "1", 0, "C", true, 0, "")
	pdf.CellFormat(70, 7, "Keterangan", "1", 0, "L", true, 0, "")
	pdf.CellFormat(25, 7, "Debit (Rp)", "1", 0, "R", true, 0, "")
	pdf.CellFormat(25, 7, "Kredit (Rp)", "1", 0, "R", true, 0, "")
	pdf.CellFormat(25, 7, "Saldo (Rp)", "1", 0, "R", true, 0, "")
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 9)
	for _, l := range lines {
		pdf.CellFormat(20, 6, l.Date.Format("2006-01-02"), "1", 0, "C", false, 0, "")
		pdf.CellFormat(20, 6, l.Reference, "1", 0, "C", false, 0, "")
		pdf.CellFormat(70, 6, truncate(l.Description, 40), "1", 0, "L", false, 0, "")
		pdf.CellFormat(25, 6, formatIDR(l.DebitMinor), "1", 0, "R", false, 0, "")
		pdf.CellFormat(25, 6, formatIDR(l.CreditMinor), "1", 0, "R", false, 0, "")
		pdf.CellFormat(25, 6, formatIDR(l.BalanceMinor), "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
	}
}

func drawFooter(pdf *gofpdf.Fpdf, s domain.Statement) {
	pdf.Ln(6)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(120, 7, "", "0", 0, "L", false, 0, "")
	pdf.CellFormat(30, 7, "Saldo Akhir:", "0", 0, "R", false, 0, "")
	pdf.CellFormat(35, 7, formatIDR(s.ClosingBalanceMinor), "1", 0, "R", false, 0, "")
	pdf.Ln(-1)
	pdf.SetFont("Helvetica", "", 9)
	pdf.MultiCell(0, 5, fmt.Sprintf(
		"Dokumen ini dicetak otomatis pada %s oleh Fledger Dunning. "+
			"Apabila terdapat selisih, mohon hubungi Finance HQ dalam 7 hari kerja.",
		time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
	), "", "L", false)
}

func formatIDR(n int64) string {
	if n == 0 {
		return "-"
	}
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "???"
}