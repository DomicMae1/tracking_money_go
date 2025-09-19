// main.go
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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

var client *mongo.Client

func connectDB() (*mongo.Client, error) {
	// Dapatkan connection string dari Environment Variable
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		return nil, fmt.Errorf("MONGO_URI environment variable not set")
	}

	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(uri).SetServerAPIOptions(serverAPI)

	// Hubungkan ke MongoDB
	c, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
		return nil, err
	}
	
	// Ping database untuk memastikan koneksi berhasil
	if err := c.Ping(context.TODO(), nil); err != nil {
		return nil, err
	}
	
	log.Println("Berhasil terhubung ke MongoDB!")
	return c, nil
}

// Handler utama yang merutekan request
func TransactionsHandler(w http.ResponseWriter, r *http.Request) {
	// Setup CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	
	var err error
	// Jika belum terhubung, buat koneksi
	if client == nil {
		client, err = connectDB()
		if err != nil {
			http.Error(w, "Gagal terhubung ke database", http.StatusInternalServerError)
			log.Printf("Error connecting to DB: %v", err)
			return
		}
	}

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
	collection := client.Database("financial_manager").Collection("transactions")
	queryParams := r.URL.Query()
	yearFilter := queryParams.Get("year")
	monthFilter := queryParams.Get("month")

	// Buat filter MongoDB (bson.M)
	filter := bson.M{}
	if yearFilter != "" {
		// Filter berdasarkan awalan tanggal (contoh: "2025-")
		dateRegex := "^" + yearFilter
		if monthFilter != "" && monthFilter != "all" {
			dateRegex += "-" + monthFilter
		}
		filter["date"] = bson.M{"$regex": dateRegex}
	}
	
	// Cari data di database
	cursor, err := collection.Find(context.TODO(), filter, options.Find().SetSort(bson.D{{Key: "date", Value: -1}}))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var results []Transaction
	if err = cursor.All(context.TODO(), &results); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Kirim array kosong jika tidak ada hasil, bukan null
	if results == nil {
		results = make([]Transaction, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}


// Handler createTransactionHandler (tidak ada perubahan)
func createTransactionHandler(w http.ResponseWriter, r *http.Request) {
	collection := client.Database("financial_manager").Collection("transactions")
	var newTransaction Transaction
	json.NewDecoder(r.Body).Decode(&newTransaction)

	// MongoDB akan otomatis membuat _id, jadi kita tidak perlu membuatnya
	result, err := collection.InsertOne(context.TODO(), newTransaction)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Ambil kembali dokumen yang baru saja dibuat untuk dikirim sebagai respons
	var createdTransaction Transaction
	collection.FindOne(context.TODO(), bson.M{"_id": result.InsertedID}).Decode(&createdTransaction)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdTransaction)
}

func deleteTransactionHandler(w http.ResponseWriter, r *http.Request) {
	collection := client.Database("financial_manager").Collection("transactions")
	path := r.URL.Path
	parts := strings.Split(path, "/")
	idStr := parts[len(parts)-1]

	// Konversi string ID dari URL menjadi ObjectID MongoDB
	objectID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	result, err := collection.DeleteOne(context.TODO(), bson.M{"_id": objectID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if result.DeletedCount == 0 {
		http.Error(w, "Transaksi tidak ditemukan", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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