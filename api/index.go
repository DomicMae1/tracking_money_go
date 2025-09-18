package api

import "net/http"

// Vercel akan memanggil fungsi Handler ini
func Handler(w http.ResponseWriter, r *http.Request) {
	// Teruskan request ke handler utama kita yang ada di main.go
	TransactionsHandler(w, r)
}