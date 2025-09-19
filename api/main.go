// main.go
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Struct dan Database In-Memory (Tidak ada perubahan)
type Transaction struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Description string             `json:"description" bson:"description"`
	Amount      float64            `json:"amount" bson:"amount"`
	Date        string             `json:"date" bson:"date"`
	Type        string             `json:"type" bson:"type"`
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
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		return nil, fmt.Errorf("MONGO_URI environment variable not set")
	}
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(uri).SetServerAPIOptions(serverAPI)
	c, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
		return nil, err
	}
	if err := c.Ping(context.TODO(), nil); err != nil {
		return nil, err
	}
	log.Println("Berhasil terhubung ke MongoDB!")
	return c, nil
}

func ensureDBConnection(w http.ResponseWriter) bool {
	var err error
	if client == nil {
		client, err = connectDB()
		if err != nil {
			http.Error(w, "Gagal terhubung ke database", http.StatusInternalServerError)
			log.Printf("Error connecting to DB: %v", err)
			return false
		}
	}
	return true
}

// Handler utama yang merutekan request
func TransactionsHandler(w http.ResponseWriter, r *http.Request) {
	// Middleware CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
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
	// Middleware CORS (tetap sama)
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

	// 1. Buat filter (Match Stage)
	matchStage := bson.D{}
	if yearFilter != "" {
		regexPattern := fmt.Sprintf("^%s", yearFilter)
		if monthFilter != "" && monthFilter != "all" {
			if len(monthFilter) == 1 {
				monthFilter = "0" + monthFilter
			}
			regexPattern = fmt.Sprintf("^%s-%s", yearFilter, monthFilter)
		}
		matchStage = append(matchStage, bson.E{Key: "date", Value: bson.M{"$regex": regexPattern}})
	}

	// 2. Buat agregasi (Group Stage)
	// Ini adalah cara paling efisien untuk menghitung total di MongoDB.
	groupStage := bson.D{
		{Key: "_id", Value: nil}, // Kelompokkan semua dokumen jadi satu
		{Key: "totalIncome", Value: bson.D{
			{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: bson.A{
					bson.D{{Key: "$eq", Value: bson.A{"$type", "income"}}},
					"$amount",
					0,
				}},
			}},
		}},
		{Key: "totalExpense", Value: bson.D{
			{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: bson.A{
					bson.D{{Key: "$eq", Value: bson.A{"$type", "expense"}}},
					"$amount",
					0,
				}},
			}},
		}},
	}

	// 3. Jalankan pipeline agregasi
	cursor, err := collection.Aggregate(context.Background(), mongo.Pipeline{
		{{Key: "$match", Value: matchStage}},
		{{Key: "$group", Value: groupStage}},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 4. Proses hasil
	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	summary := Summary{}
	if len(results) > 0 {
		data := results[0]
		if income, ok := data["totalIncome"].(float64); ok {
			summary.TotalIncome = income
		}
		if expense, ok := data["totalExpense"].(float64); ok {
			summary.TotalExpense = expense
		}
		summary.Balance = summary.TotalIncome - summary.TotalExpense
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

func getTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	yearFilter := queryParams.Get("year")
	monthFilter := queryParams.Get("month")

	// 1. Buat filter (query) untuk MongoDB
	filter := bson.M{}
	if yearFilter != "" {
		// Filter berdasarkan tahun, contoh: "2025-"
		regexPattern := fmt.Sprintf("^%s", yearFilter)
		if monthFilter != "" && monthFilter != "all" {
			if len(monthFilter) == 1 {
				monthFilter = "0" + monthFilter // Pastikan format 2 digit, misal '09'
			}
			// Filter berdasarkan tahun dan bulan, contoh: "2025-09"
			regexPattern = fmt.Sprintf("^%s-%s", yearFilter, monthFilter)
		}
		filter["date"] = bson.M{"$regex": regexPattern}
	}

	// 2. Cari data di MongoDB
	// Opsi untuk mengurutkan data dari yang terbaru (berdasarkan _id)
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: -1}})
	cursor, err := collection.Find(context.Background(), filter, opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	// 3. Decode hasilnya ke dalam slice Transaction
	var transactions []Transaction
	if err = cursor.All(context.Background(), &transactions); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Jika tidak ada data, kirim slice kosong agar JSON menjadi `[]` bukan `null`
	if transactions == nil {
		transactions = make([]Transaction, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(transactions)
}


// Handler createTransactionHandler (tidak ada perubahan)
func createTransactionHandler(w http.ResponseWriter, r *http.Request) {
	var newTransaction Transaction
	if err := json.NewDecoder(r.Body).Decode(&newTransaction); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validasi Tipe
	if newTransaction.Type != "income" && newTransaction.Type != "expense" {
		http.Error(w, "Tipe transaksi tidak valid. Harus 'income' atau 'expense'", http.StatusBadRequest)
		return
	}

	// ID akan dibuat otomatis oleh MongoDB, jadi kita tidak perlu set manual.
	// Cukup kosongkan saja.
	newTransaction.ID = primitive.NilObjectID

	// 1. Masukkan data ke MongoDB
	result, err := collection.InsertOne(context.Background(), newTransaction)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. Set ID yang baru dibuat oleh MongoDB ke struct untuk dikirim kembali
	newTransaction.ID = result.InsertedID.(primitive.ObjectID)

	log.Printf("Transaksi baru disimpan dengan ID: %s", newTransaction.ID.Hex())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newTransaction)
}

func deleteTransactionHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	parts := strings.Split(path, "/")
	idStr := parts[len(parts)-1]

	// 1. Konversi string ID dari URL menjadi ObjectID MongoDB
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	// 2. Hapus dokumen dari MongoDB berdasarkan _id
	result, err := collection.DeleteOne(context.Background(), bson.M{"_id": id})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Cek apakah ada dokumen yang terhapus
	if result.DeletedCount == 0 {
		http.Error(w, fmt.Sprintf("Transaksi dengan ID %s tidak ditemukan", idStr), http.StatusNotFound)
		return
	}

	log.Printf("Transaksi dengan ID %s telah dihapus.", idStr)
	w.WriteHeader(http.StatusNoContent)
}

func monthlySummaryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	if r.Method == "OPTIONS" { w.WriteHeader(http.StatusOK); return }
	
	if !ensureDBConnection(w) { return }
	collection := client.Database("financial_manager").Collection("transactions")

	queryParams := r.URL.Query()
	yearFilter := queryParams.Get("year")
	if yearFilter == "" {
		http.Error(w, "Parameter 'year' dibutuhkan", http.StatusBadRequest)
		return
	}

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "date", Value: bson.M{"$regex": "^" + yearFilter}}}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "month", Value: bson.D{{Key: "$substr", Value: bson.A{"$date", 5, 2}}}},
				{Key: "type", Value: "$type"},
			}},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: "$amount"}}},
		}}},
	}

	cursor, err := collection.Aggregate(context.TODO(), pipeline)
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }

	var results []bson.M
	if err = cursor.All(context.TODO(), &results); err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }

	// Proses hasil agregasi
	monthlyData := make(map[string]map[string]float64)
	for _, result := range results {
		id := result["_id"].(primitive.D)
		month := id.Map()["month"].(string)
		transType := id.Map()["type"].(string)
		total := result["total"].(float64)
		
		if _, ok := monthlyData[month]; !ok {
			monthlyData[month] = make(map[string]float64)
		}
		monthlyData[month][transType] = total
	}

	// Format hasil akhir
	var finalResult []MonthlySummary
	monthNames := []string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
	for i := 1; i <= 12; i++ {
		monthStr := fmt.Sprintf("%02d", i)
		data := monthlyData[monthStr]
		finalResult = append(finalResult, MonthlySummary{
			Month:   monthNames[i-1],
			Income:  data["income"],
			Expense: data["expense"],
		})
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalResult)
	fmt.Fprintf(w, "Endpoint ini perlu diimplementasikan dengan MongoDB Aggregation.")
}

func main() {
	http.HandleFunc("/api/transactions", TransactionsHandler)
	http.HandleFunc("/api/transactions/", deleteTransactionHandler) // Perhatikan '/' di akhir
	http.HandleFunc("/api/summary", SummaryHandler)
	http.HandleFunc("/api/monthly-summary", monthlySummaryHandler)

	fmt.Println("🚀 Server berjalan di port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}