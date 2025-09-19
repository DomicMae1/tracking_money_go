// api/index.go
package api

import (
	"net/http"
	"strings"
)

// Handler utama yang akan dipanggil oleh Vercel
func Handler(w http.ResponseWriter, r *http.Request) {
	// ================== LOGIKA CORS DITAMBAHKAN DI SINI ==================
	// Memberikan "izin" ke semua domain (*). Untuk keamanan lebih,
	// Anda bisa mengganti "*" dengan "http://localhost:5173" saat development.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	
	// Tangani "preflight request" (OPTIONS) yang dikirim browser sebelum
	// request POST atau DELETE. Ini sangat penting.
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	// ====================================================================

	// Routing sederhana berdasarkan path URL (logika ini tetap sama)
	if strings.HasPrefix(r.URL.Path, "/api/monthly-summary") {
		monthlySummaryHandler(w, r)
	} else if strings.HasPrefix(r.URL.Path, "/api/summary") {
		SummaryHandler(w, r)
	} else if strings.HasPrefix(r.URL.Path, "/api/transactions") {
		TransactionsHandler(w, r)
	} else {
		// Jika path tidak dikenali, kirim 404 Not Found
		http.NotFound(w, r)
	}
}