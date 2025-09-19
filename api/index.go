// api/index.go
package api

import (
	"net/http"
	"strings"
)

// Handler utama yang akan dipanggil oleh Vercel
func Handler(w http.ResponseWriter, r *http.Request) {
	// Routing sederhana berdasarkan path URL
	if strings.HasPrefix(r.URL.Path, "/api/monthly-summary") {
		monthlySummaryHandler(w, r)
	} else if strings.HasPrefix(r.URL.Path, "/api/summary") {
		SummaryHandler(w, r)
	} else if strings.HasPrefix(r.URL.Path, "/api/transactions") {
		TransactionsHandler(w, r)
	} else {
		http.NotFound(w, r)
	}
}
