package project

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"

	"github.com/gorilla/mux"

	"github.com/joho/godotenv"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

/*========== GLOBALS ==========*/
var (
	db        *sql.DB
	rdb       *redis.Client
	ctx       = context.Background()
	jwtSecret []byte
)

/*========== MODELS ==========*/
type Department struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Employee struct {
	ID           int        `json:"id"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	Phone        string     `json:"phone"`
	Salary       float64    `json:"salary"`
	DepartmentID int        `json:"department_id"`
	Status       string     `json:"status"`
	CreatedAt    *time.Time `json:"created_at"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Claims struct {
	UserID int    `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

/*========== CONNECTIONS ==========*/
func connectDB() {
	var err error

	dsn := os.Getenv("MySQL_DSN")
	if dsn == "" {
		log.Fatal("MySQL_DSN not set in environment")
	}

	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal("Database not reachable", err)
	}
	log.Println("Database connected successfully.")
}

func connectrRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis:%v", err)
	}
	log.Println("Redis connected successfully.")
}

/*========== JWT ==========*/
func generateJWT(userID int, email string) (string, time.Duration, error) {
	expiry := time.Minute * 15
	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(jwtSecret)
	return signed, expiry, err
}

func jwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		auth := r.Header.Get("Authorization")
		parts := strings.Split(auth, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		tokenStr := parts[1]

		if rdb.Exists(ctx, "bl:"+tokenStr).Val() == 1 {
			http.Error(w, "Token revoked", http.StatusUnauthorized)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "email", claims.Email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

/*========== AUTH ==========*/
func login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	var userID int
	var hash string
	err = db.QueryRow(
		"SELECT id,password FROM users WHERE email=?", req.Email,
	).Scan(&userID, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	token, ttl, err := generateJWT(userID, req.Email)
	if err != nil {
		http.Error(w, "Token generation failed", http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(map[string]string{
		"access_token": token,
		"expires_in":   ttl.String(),
	})
}

func logout(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.Header.Get("Authorization"), " ")
	if len(parts) != 2 {
		http.Error(w, "Invalid token", 401)
		return
	}
	token := parts[1]

	rdb.Set(ctx, "bl:"+token, "revoked", time.Minute*15)
	w.Write([]byte("Logged out"))
}

/*========== HANDLERS ==========*/
func createDepartment(w http.ResponseWriter, r *http.Request) {
	var dept Department

	err := json.NewDecoder(r.Body).Decode(&dept)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := departmentValidation(dept); err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}

	query := `INSERT INTO departments(name) VALUES(?)`
	result, err := db.Exec(query, dept.Name)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error inserting:%v", err), http.StatusInternalServerError)
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	dept.ID = int(id)

	rdb.Del(ctx, "departments:all")

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(dept)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
}

func getDepartments(w http.ResponseWriter, r *http.Request) {
	cacheKey := "departments:all"

	val, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		log.Println("Cache hit...")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(val))
		return
	}

	fmt.Println("Missing cache, quering DB...")
	result, err := db.Query("SELECT * FROM departments")
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	defer result.Close()

	var departments []Department
	for result.Next() {
		var d Department
		result.Scan(&d.ID, &d.Name)
		departments = append(departments, d)
	}

	data, err := json.Marshal(departments)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	rdb.Set(ctx, cacheKey, data, 10*time.Minute)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func createEmployee(w http.ResponseWriter, r *http.Request) {

	var emp Employee

	err := json.NewDecoder(r.Body).Decode(&emp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}

	if err := employeeValidation(emp); err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}

	query := `INSERT INTO employees(name,email,phone,salary,department_id,status) VALUES (?,?,?,?,?,?)`
	result, err := db.Exec(query,
		emp.Name, emp.Email,
		emp.Phone,
		emp.Salary, emp.DepartmentID,
		emp.Status)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	emp.ID = int(id)

	rdb.Del(ctx, "employees:all")

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(emp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
}

func getEmployees(w http.ResponseWriter, r *http.Request) {
	cacheKey := "employees:all"

	if val, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
		log.Println("Cache hit...")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(val))
		return
	}

	fmt.Println("Missing cache, quering DB...")
	rows, err := db.Query("SELECT * FROM employees")
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var employees []Employee
	for rows.Next() {
		var e Employee
		rows.Scan(
			&e.ID,
			&e.Name,
			&e.Email,
			&e.Phone,
			&e.Salary,
			&e.DepartmentID,
			&e.Status,
			&e.CreatedAt,
		)
		employees = append(employees, e)
	}

	data, err := json.Marshal(employees)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	rdb.Set(ctx, cacheKey, data, 10*time.Minute)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func getEmployeeByID(w http.ResponseWriter, r *http.Request) {

	id := mux.Vars(r)["id"]
	cacheKey := fmt.Sprintf("employee:%s", id)

	if val, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
		log.Println("Cache hit...")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(val))
		return
	}

	fmt.Println("Missing cache, quering DB...")
	var e Employee
	query :=
		`SELECT * FROM employees WHERE id=?`
	err := db.QueryRow(query, id).Scan(
		&e.ID,
		&e.Name,
		&e.Email,
		&e.Phone, &e.Salary,
		&e.DepartmentID,
		&e.Status,
		&e.CreatedAt)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(e)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	rdb.Set(ctx, cacheKey, data, 10*time.Minute)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func updateEmployee(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var emp Employee

	err := json.NewDecoder(r.Body).Decode(&emp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}

	if err := employeeValidation(emp); err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}

	query := `
	UPDATE employees SET 
	name=?,email=?,phone=?,salary=?,department_id=?,status=?
	WHERE id=?`

	_, err1 := db.Exec(
		query,
		&emp.Name,
		&emp.Email,
		&emp.Phone,
		&emp.Salary,
		&emp.DepartmentID,
		&emp.Status,
		id,
	)
	if err1 != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err1), http.StatusInternalServerError)
		return
	}

	rdb.Del(ctx, "employees:all")
	rdb.Del(ctx, fmt.Sprintf("employee:%s", id))
	w.Write([]byte("Employee Updated Successfully."))
}

func deleteEmployee(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	_, err := db.Exec("DELETE FROM employees WHERE id=?", id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}

	rdb.Del(ctx, "employees:all")
	rdb.Del(ctx, fmt.Sprintf("employee:%s", id))
	w.Write([]byte("Deleted employee successfully."))
}

func departmentValidation(d Department) error {

	if strings.TrimSpace(d.Name) == "" {
		return errors.New("Department name cannot be Empty.")
	}
	if len(d.Name) < 2 {
		return errors.New("Department name cannot be less than 2 Characters")
	}
	return nil
}

func employeeValidation(e Employee) error {
	if strings.TrimSpace(e.Name) == "" {
		return errors.New("Employee name cannot be empty")
	}
	if !strings.Contains(e.Email, "@") {
		return errors.New("Email syntax is Invalid")
	}
	if !strings.HasSuffix(e.Email, "@gmail.com") {
		return errors.New("Invalid Email. Must contain '@gmail.com'")
	}
	prefix := strings.TrimSuffix(e.Email, "@gmail.com")
	if prefix == "" {
		return errors.New("Email must contain a prefix to @gmail.com")
	}
	if strings.TrimSpace(e.Phone) == "" {
		return errors.New("Phone number is required")
	}
	if e.Salary < 0 {
		return errors.New("Salary cannot be less than zero")
	}
	if e.DepartmentID < 0 {
		return errors.New("Invalid department ID")
	}
	if strings.TrimSpace(e.Status) == "" {
		return errors.New("Status cannot be empty")
	}
	return nil
}

/*========== MAIN ==========*/
func EmsHandler() {

	godotenv.Load()
	jwtSecret = []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		log.Fatal("JWT_SECRET missing")
	}

	connectDB()
	connectrRedis()

	router := mux.NewRouter()

	router.HandleFunc("/login", login).Methods("POST")

	protected := router.PathPrefix("/").Subrouter()
	protected.Use(jwtMiddleware)

	protected.HandleFunc("/logout", logout).Methods("POST")

	router.HandleFunc("/departments", createDepartment).Methods("POST")
	router.HandleFunc("/departments", getDepartments).Methods("GET")

	router.HandleFunc("/employees", createEmployee).Methods("POST")
	router.HandleFunc("/employees", getEmployees).Methods("GET")
	router.HandleFunc("/employees/{id}", getEmployeeByID).Methods("GET")
	router.HandleFunc("/employees/{id}", updateEmployee).Methods("PUT")
	router.HandleFunc("/employees/{id}", deleteEmployee).Methods("DELETE")

	log.Println("Server running on port:8080")
	http.ListenAndServe(":8080", router)
}
