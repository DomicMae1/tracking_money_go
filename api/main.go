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

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

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
	UserID      primitive.ObjectID `json:"userId" bson:"userId"`
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

type User struct {
	ID       primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Username string             `json:"username" bson:"username"`
	Email    string             `json:"email" bson:"email"`
	Password string             `json:"password,omitempty" bson:"password"`
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

//Registrasi
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if !ensureDBConnection(w) { return }
	collection := client.Database("financial_manager").Collection("users")

	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	// hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Error hashing password", http.StatusInternalServerError)
		return
	}
	user.Password = string(hash)
	user.ID = primitive.NewObjectID()

	_, err = collection.InsertOne(context.TODO(), user)
	if err != nil {
		http.Error(w, "User already exists", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "User registered successfully"})
}

//Login
var jwtSecret = []byte("SECRET_KEY_GANTI") // sebaiknya ambil dari ENV

// Contoh bikin token JWT
func GenerateToken(username string) (string, error) {
	claims := &jwt.RegisteredClaims{
		Subject:   username,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // <- pakai time
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// Contoh hash password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost) // <- pakai bcrypt
	return string(bytes), err
}

// Contoh cek password
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if !ensureDBConnection(w) { return }
	collection := client.Database("financial_manager").Collection("users")

	var input User
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	var user User
	err := collection.FindOne(context.TODO(), bson.M{"email": input.Email}).Decode(&user)
	if err != nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	// cek password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	// buat JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": user.ID.Hex(),
		"exp":    time.Now().Add(time.Hour * 24).Unix(),
	})
	tokenString, _ := token.SignedString(jwtSecret)

	json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
}

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing token", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Ambil claim userId dari token
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "Invalid claims", http.StatusUnauthorized)
			return
		}

		userIdStr, ok := claims["userId"].(string)
		if !ok {
			http.Error(w, "Invalid token payload", http.StatusUnauthorized)
			return
		}

		// simpan ke context agar bisa diakses handler
		ctx := context.WithValue(r.Context(), "userId", userIdStr)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
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
	// Ambil userId dari context (hasil middleware auth)
	userIdStr := r.Context().Value("userId").(string)
	userId, _ := primitive.ObjectIDFromHex(userIdStr)

	// Mulai filter dengan userId
	filter := bson.M{"userId": userId}

	// Tambahkan filter tahun & bulan
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
	if !ensureDBConnection(w) {
		return
	}
	collection := client.Database("financial_manager").Collection("transactions")

	queryParams := r.URL.Query()
	yearFilter := queryParams.Get("year")
	if yearFilter == "" {
		http.Error(w, "Parameter 'year' dibutuhkan", http.StatusBadRequest)
		return
	}
	monthFilter := queryParams.Get("month")

	// 🔥 Ambil userId dari context (hasil AuthMiddleware)
	userIdStr := r.Context().Value("userId").(string)
	userId, _ := primitive.ObjectIDFromHex(userIdStr)

	// === Filter dasar: hanya transaksi milik user ini ===
	match := bson.D{
		{Key: "userId", Value: userId},
		{Key: "date", Value: bson.M{"$regex": "^" + yearFilter}},
	}

	if monthFilter != "" && monthFilter != "all" {
		// contoh: "2025-09"
		prefix := fmt.Sprintf("^%s-%s", yearFilter, monthFilter)
		match = bson.D{
			{Key: "userId", Value: userId},
			{Key: "date", Value: bson.M{"$regex": prefix}},
		}
	}

	// === Pipeline dengan filter userId ===
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "month", Value: bson.D{{Key: "$substr", Value: bson.A{"$date", 5, 2}}}},
				{Key: "type", Value: "$type"},
			}},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: "$amount"}}},
		}}},
	}

	cursor, err := collection.Aggregate(context.TODO(), pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var results []bson.M
	if err = cursor.All(context.TODO(), &results); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// === Proses hasil ===
	monthlyData := make(map[string]map[string]float64)
	for _, result := range results {
		idDoc, ok := result["_id"].(bson.M)
		if !ok {
			continue
		}

		month, _ := idDoc["month"].(string)
		transType, _ := idDoc["type"].(string)

		var total float64
		switch v := result["total"].(type) {
		case float64:
			total = v
		case int32:
			total = float64(v)
		case int64:
			total = float64(v)
		case primitive.Decimal128:
			total, _ = strconv.ParseFloat(v.String(), 64)
		}

		if _, ok := monthlyData[month]; !ok {
			monthlyData[month] = make(map[string]float64)
		}
		monthlyData[month][transType] = total
	}

	// === Format output ===
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
}

func getTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	collection := client.Database("financial_manager").Collection("transactions")

	// 🔥 ambil userId dari context
	userIdStr := r.Context().Value("userId").(string)
	userId, _ := primitive.ObjectIDFromHex(userIdStr)

	queryParams := r.URL.Query()
	yearFilter := queryParams.Get("year")
	monthFilter := queryParams.Get("month")

	filter := bson.M{"userId": userId} // 🔥 filter hanya data milik user
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
	collection := client.Database("financial_manager").Collection("transactions")

	var newTransaction Transaction
	json.NewDecoder(r.Body).Decode(&newTransaction)

	if newTransaction.Type != "income" && newTransaction.Type != "expense" {
		http.Error(w, "Tipe transaksi tidak valid", http.StatusBadRequest)
		return
	}

	// 🔥 tambahkan userId ke transaksi
	userIdStr := r.Context().Value("userId").(string)
	userId, _ := primitive.ObjectIDFromHex(userIdStr)
	newTransaction.UserID = userId

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
	collection := client.Database("financial_manager").Collection("transactions")

	path := r.URL.Path
	parts := strings.Split(path, "/")
	idStr := parts[len(parts)-1]
	objectID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil { http.Error(w, "ID tidak valid", http.StatusBadRequest); return }

	// 🔥 hanya boleh hapus milik sendiri
	userIdStr := r.Context().Value("userId").(string)
	userId, _ := primitive.ObjectIDFromHex(userIdStr)

	result, err := collection.DeleteOne(context.TODO(), bson.M{
		"_id":    objectID,
		"userId": userId,
	})
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	if result.DeletedCount == 0 { http.Error(w, "Transaksi tidak ditemukan", http.StatusNotFound); return }
	w.WriteHeader(http.StatusNoContent)
}