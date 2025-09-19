// main.go
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Struct dan Database In-Memory (Tidak ada perubahan)
type Transaction struct {
	ID          int     `json:"id"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Date        string  `json:"date"`
	Type        string  `json:"type"`
}

type Summary struct {
	TotalIncome  float64 `json:"totalIncome"`
	TotalExpense float64 `json:"totalExpense"`
	Balance      float64 `json:"balance"`
}

type MonthlySummary struct {
	Month   string  `json:"month"`
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
}

var transactions = []Transaction{
	{ID: 1, Description: "Gaji Bulanan", Amount: 5000000, Date: "2025-09-01", Type: "income"},
	{ID: 2, Description: "Belanja Bulanan", Amount: 750000, Date: "2025-09-05", Type: "expense"},
	{ID: 3, Description: "Nasi Padang", Amount: 30000, Date: "2025-09-17", Type: "expense"},
	{ID: 4, Description: "Kopi Pagi", Amount: 25000, Date: "2025-09-18", Type: "expense"},
}
var nextID = 5

// Handler utama yang merutekan request
func TransactionsHandler(w http.ResponseWriter, r *http.Request) {
	// Middleware CORS sekarang ditempatkan di sini
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Routing berdasarkan metode HTTP
	switch r.Method {
	case "GET":
		getTransactionsHandler(w, r)
	case "POST":
		createTransactionHandler(w, r)
	case "DELETE":
		deleteTransactionHandler(w, r)
	default:
		http.Error(w, "Metode tidak diizinkan", http.StatusMethodNotAllowed)
	}
}


// Handler GET, POST, DELETE (Tidak ada perubahan sama sekali di dalamnya)
// main.go -> Ganti FUNGSI INI SAJA

func SummaryHandler(w http.ResponseWriter, r *http.Request) {
	// Middleware CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	queryParams := r.URL.Query()
	yearFilter := queryParams.Get("year")
	monthFilter := queryParams.Get("month")

	var totalIncome, totalExpense float64

	for _, t := range transactions {
		transactionDate, err := time.Parse("2006-01-02", t.Date)
		if err != nil {
			continue
		}
		yearMatch, monthMatch := true, true
		if yearFilter != "" {
			year, _ := strconv.Atoi(yearFilter)
			if transactionDate.Year() != year {
				yearMatch = false
			}
		}
		if monthFilter != "" && monthFilter != "all" {
			month, _ := strconv.Atoi(monthFilter)
			if int(transactionDate.Month()) != month {
				monthMatch = false
			}
		}

		if yearMatch && monthMatch {
			if t.Type == "income" {
				totalIncome += t.Amount
			} else if t.Type == "expense" {
				totalExpense += t.Amount
			}
		}
	}

	summary := Summary{
		TotalIncome:  totalIncome,
		TotalExpense: totalExpense,
		Balance:      totalIncome - totalExpense,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

func getTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Mengambil data transaksi (dengan potensi filter)...")

	queryParams := r.URL.Query()
	yearFilter := queryParams.Get("year")
	monthFilter := queryParams.Get("month")

	// Mulai dengan semua transaksi, lalu kita akan filter jika perlu
	filteredTransactions := transactions

	// Terapkan filter tahun jika ada
	if yearFilter != "" {
		var tempTransactions []Transaction
		year, _ := strconv.Atoi(yearFilter)
		for _, t := range filteredTransactions {
			transactionDate, err := time.Parse("2006-01-02", t.Date)
			if err != nil {
				continue
			}
			if transactionDate.Year() == year {
				tempTransactions = append(tempTransactions, t)
			}
		}
		filteredTransactions = tempTransactions
	}
	
	// Terapkan filter bulan pada hasil yang sudah difilter tahun (jika ada)
	if monthFilter != "" && monthFilter != "all" {
		var tempTransactions []Transaction
		month, _ := strconv.Atoi(monthFilter)
		for _, t := range filteredTransactions {
			transactionDate, err := time.Parse("2006-01-02", t.Date)
			if err != nil {
				continue
			}
			if int(transactionDate.Month()) == month {
				tempTransactions = append(tempTransactions, t)
			}
		}
		filteredTransactions = tempTransactions
	}

	if filteredTransactions == nil {
		filteredTransactions = make([]Transaction, 0) // Buat slice kosong, bukan nil
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filteredTransactions)
}


// Handler createTransactionHandler (tidak ada perubahan)
func createTransactionHandler(w http.ResponseWriter, r *http.Request) {
	var newTransaction Transaction
	err := json.NewDecoder(r.Body).Decode(&newTransaction)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Validasi sederhana untuk 'Type'
	if newTransaction.Type != "income" && newTransaction.Type != "expense" {
		http.Error(w, "Tipe transaksi tidak valid. Harus 'income' atau 'expense'", http.StatusBadRequest)
		return
	}
	newTransaction.ID = nextID
	nextID++
	transactions = append([]Transaction{newTransaction}, transactions...)
	log.Printf("Transaksi baru diterima: %+v\n", newTransaction)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newTransaction)
}

func deleteTransactionHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Dapatkan ID dari URL, contoh: /api/transactions/1 -> "1"
	path := r.URL.Path
	parts := strings.Split(path, "/")
	idStr := parts[len(parts)-1] // Ambil bagian terakhir dari URL

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	// 2. Cari index dari transaksi yang akan dihapus
	indexToDelete := -1
	for i, t := range transactions {
		if t.ID == id {
			indexToDelete = i
			break
		}
	}

	// 3. Jika ditemukan, hapus dari slice. Jika tidak, kirim error 404
	if indexToDelete != -1 {
		// Cara standar untuk menghapus elemen dari slice di Go
		transactions = append(transactions[:indexToDelete], transactions[indexToDelete+1:]...)
		log.Printf("Transaksi dengan ID %d telah dihapus.", id)
		w.WriteHeader(http.StatusNoContent) // Status 204 No Content, artinya sukses tapi tidak ada body respons
	} else {
		http.Error(w, fmt.Sprintf("Transaksi dengan ID %d tidak ditemukan", id), http.StatusNotFound)
	}
}

func monthlySummaryHandler(w http.ResponseWriter, r *http.Request) {
	// Middleware CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	queryParams := r.URL.Query()
	yearFilter := queryParams.Get("year")
	if yearFilter == "" {
		http.Error(w, "Parameter 'year' dibutuhkan", http.StatusBadRequest)
		return
	}
	year, _ := strconv.Atoi(yearFilter)

	// Inisialisasi data untuk 12 bulan
	monthlyData := make(map[time.Month]struct {
		Income  float64
		Expense float64
	})

	for _, t := range transactions {
		transactionDate, err := time.Parse("2006-01-02", t.Date)
		if err != nil {
			continue
		}

		if transactionDate.Year() == year {
			month := transactionDate.Month()
			data := monthlyData[month]
			if t.Type == "income" {
				data.Income += t.Amount
			} else if t.Type == "expense" {
				data.Expense += t.Amount
			}
			monthlyData[month] = data
		}
	}

	// Format hasil ke dalam slice MonthlySummary
	var result []MonthlySummary
	months := []time.Month{time.January, time.February, time.March, time.April, time.May, time.June, time.July, time.August, time.September, time.October, time.November, time.December}
	monthNames := []string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
	
	for i, month := range months {
		data := monthlyData[month]
		result = append(result, MonthlySummary{
			Month:   monthNames[i],
			Income:  data.Income,
			Expense: data.Expense,
		})
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
