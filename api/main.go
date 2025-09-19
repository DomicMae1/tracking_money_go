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
	if uri == "" { return nil, fmt.Errorf("MONGO_URI environment variable not set") }
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(uri).SetServerAPIOptions(serverAPI)
	c, err := mongo.Connect(context.TODO(), opts)
	if err != nil { return nil, err }
	if err := c.Ping(context.TODO(), nil); err != nil { return nil, err }
	log.Println("Berhasil terhubung ke MongoDB!")
	return c, nil
}

// Fungsi helper untuk memastikan koneksi DB siap
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
	if !ensureDBConnection(w) { return }
	switch r.Method {
	case "GET": getTransactionsHandler(w, r)
	case "POST": createTransactionHandler(w, r)
	case "DELETE": deleteTransactionHandler(w, r)
	default: http.Error(w, "Metode tidak diizinkan", http.StatusMethodNotAllowed)
	}
}


// Handler GET, POST, DELETE (Tidak ada perubahan sama sekali di dalamnya)
// main.go -> Ganti FUNGSI INI SAJA

func SummaryHandler(w http.ResponseWriter, r *http.Request) {
	if !ensureDBConnection(w) { return }
	// === PERBAIKAN: Deklarasikan 'collection' ===
	collection := client.Database("financial_manager").Collection("transactions")

	queryParams := r.URL.Query()
	yearFilter := queryParams.Get("year")
	monthFilter := queryParams.Get("month")
	filter := bson.M{}
	if yearFilter != "" {
		dateRegex := "^" + yearFilter
		if monthFilter != "" && monthFilter != "all" {
			dateRegex += "-" + monthFilter
		}
		filter["date"] = bson.M{"$regex": dateRegex}
	}
	
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: filter}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$type"},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: "$amount"}}},
		}}},
	}
	
	cursor, err := collection.Aggregate(context.TODO(), pipeline)
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }

	var results []bson.M
	if err = cursor.All(context.TODO(), &results); err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }

	summary := Summary{}
	for _, result := range results {
		if result["_id"] == "income" {
			summary.TotalIncome = result["total"].(float64)
		} else if result["_id"] == "expense" {
			summary.TotalExpense = result["total"].(float64)
		}
	}
	summary.Balance = summary.TotalIncome - summary.TotalExpense
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

func monthlySummaryHandler(w http.ResponseWriter, r *http.Request) {
	if !ensureDBConnection(w) { return }
	collection := client.Database("financial_manager").Collection("transactions")

	queryParams := r.URL.Query()
	yearFilter := queryParams.Get("year")
	if yearFilter == "" {
		http.Error(w, "Parameter 'year' dibutuhkan", http.StatusBadRequest)
		return
	}

	// === PIPELINE AGREGRASI YANG DIPERBAIKI (KEMBALI KE STRATEGI STRING) ===
	pipeline := mongo.Pipeline{
		// Tahap 1: Filter dokumen yang cocok dengan tahun yang diberikan menggunakan regex.
		// Ini efisien jika field 'date' diindeks.
		bson.D{{Key: "$match", Value: bson.D{{Key: "date", Value: bson.M{"$regex": "^" + yearFilter}}}}},

		// Tahap 2: Kelompokkan berdasarkan bulan (diambil dari string) dan tipe.
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				// Ambil 2 karakter setelah karakter ke-5 (indeks 5, panjang 2) -> "MM"
				{Key: "month", Value: bson.D{{Key: "$substr", Value: bson.A{"$date", 5, 2}}}},
				{Key: "type", Value: "$type"},
			}},
			// Jumlahkan totalnya
			{Key: "total", Value: bson.D{{Key: "$sum", Value: "$amount"}}},
		}}},
	}

	cursor, err := collection.Aggregate(context.TODO(), pipeline)
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	defer cursor.Close(context.TODO())

	var results []bson.M
	if err = cursor.All(context.TODO(), &results); err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }

	// Proses hasil agregasi dengan aman (kode ini sudah benar)
	monthlyData := make(map[string]map[string]float64)
	for _, result := range results {
		idDoc, ok := result["_id"].(primitive.D)
		if !ok { continue }
		idMap := idDoc.Map()

		monthVal, ok := idMap["month"]
		if !ok { continue }
		month, ok := monthVal.(string)
		if !ok { continue }
		
		typeVal, ok := idMap["type"]
		if !ok { continue }
		transType, ok := typeVal.(string)
		if !ok { continue }

		var total float64
		if totalVal, ok := result["total"]; ok {
			switch v := totalVal.(type) {
			case float64: total = v
			case int32:   total = float64(v)
			case int64:   total = float64(v)
			case primitive.Decimal128:
				total, _ = strconv.ParseFloat(v.String(), 64)
			}
		}
		
		if _, ok := monthlyData[month]; !ok {
			monthlyData[month] = make(map[string]float64)
		}
		monthlyData[month][transType] = total
	}

	// Format hasil akhir (kode ini sudah benar)
	var finalResult []MonthlySummary
	monthNames := []string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
	for i := 1; i <= 12; i++ {
		monthStr := fmt.Sprintf("%02d", i) // "01", "02", ..., "09", "10", ...
		data := monthlyData[monthStr]
		finalResult = append(finalResult, MonthlySummary{
			Month:   monthNames[i-1],
			Income:  data["income"],
			Expense: data["expense"],
		})
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalResult)
}

func getTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	// === PERBAIKAN: Deklarasikan 'collection' ===
	collection := client.Database("financial_manager").Collection("transactions")

	queryParams := r.URL.Query()
	yearFilter := queryParams.Get("year")
	monthFilter := queryParams.Get("month")
	filter := bson.M{}
	if yearFilter != "" {
		dateRegex := "^" + yearFilter
		if monthFilter != "" && monthFilter != "all" {
			dateRegex += "-" + monthFilter
		}
		filter["date"] = bson.M{"$regex": dateRegex}
	}
	
	sortOrder := bson.D{{Key: "date", Value: -1}}
	cursor, err := collection.Find(context.TODO(), filter, options.Find().SetSort(sortOrder))
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	defer cursor.Close(context.TODO())

	var results []Transaction
	if err = cursor.All(context.TODO(), &results); err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	if results == nil { results = make([]Transaction, 0) }
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// Handler createTransactionHandler (tidak ada perubahan)
func createTransactionHandler(w http.ResponseWriter, r *http.Request) {
	// === PERBAIKAN: Deklarasikan 'collection' ===
	collection := client.Database("financial_manager").Collection("transactions")
	
	var newTransaction Transaction
	json.NewDecoder(r.Body).Decode(&newTransaction)
	if newTransaction.Type != "income" && newTransaction.Type != "expense" {
		http.Error(w, "Tipe transaksi tidak valid", http.StatusBadRequest)
		return
	}
	
	newTransaction.ID = primitive.NewObjectID()
	result, err := collection.InsertOne(context.TODO(), newTransaction)
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	
	var createdTransaction Transaction
	collection.FindOne(context.TODO(), bson.M{"_id": result.InsertedID}).Decode(&createdTransaction)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdTransaction)
}

func deleteTransactionHandler(w http.ResponseWriter, r *http.Request) {
	// === PERBAIKAN: Deklarasikan 'collection' ===
	collection := client.Database("financial_manager").Collection("transactions")

	path := r.URL.Path
	parts := strings.Split(path, "/")
	idStr := parts[len(parts)-1]
	objectID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil { http.Error(w, "ID tidak valid", http.StatusBadRequest); return }
	result, err := collection.DeleteOne(context.TODO(), bson.M{"_id": objectID})
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	if result.DeletedCount == 0 { http.Error(w, "Transaksi tidak ditemukan", http.StatusNotFound); return }
	w.WriteHeader(http.StatusNoContent)
}