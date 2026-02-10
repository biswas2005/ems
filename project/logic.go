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

	//MySQL driver
	_ "github.com/go-sql-driver/mysql"
	//JWT library
	"github.com/golang-jwt/jwt/v5"
	//HTTP router
	"github.com/gorilla/mux"
	//Load environment variables
	"github.com/joho/godotenv"
	//Redis client
	"github.com/redis/go-redis/v9"
	//Password hashing
	"golang.org/x/crypto/bcrypt"
)

/*========== GLOBALS ==========*/
//Global shared resources
var (
	db        *sql.DB
	rdb       *redis.Client
	ctx       = context.Background()
	jwtSecret []byte
)

/*========== MODELS ==========*/
//Department entity
type Department struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Employee entity
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

// Login request payload
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// JWT claims structure
type Claims struct {
	UserID int    `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

/*========== CONNECTIONS ==========*/
//Connect to MySQL database
func connectDB() {
	var err error
	//Read DSN from environment
	dsn := os.Getenv("MySQL_DSN")
	if dsn == "" {
		log.Fatal("MySQL_DSN not set in environment")
	}

	//Open database connection
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	//Verify connection
	err = db.Ping()
	if err != nil {
		log.Fatal("Database not reachable", err)
	}
	log.Println("Database connected successfully.")
}

// Connect to Redis
func connectrRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	//Test Redis connection
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis:%v", err)
	}
	log.Println("Redis connected successfully.")
}

/*========== JWT ==========*/
//Generate a JWT access token
func generateJWT(userID int, email string) (string, time.Duration, error) {
	expiry := time.Minute * 15
	//Create claims
	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	//Create token with HMAC SHA256
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	//Sign token
	signed, err := token.SignedString(jwtSecret)
	return signed, expiry, err
}

// JWT authentication middleware
func jwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		//Read Authorization header
		auth := r.Header.Get("Authorization")
		parts := strings.Split(auth, " ")
		//Expect:Bearer <Token>
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		tokenStr := parts[1]
		//Check Redis blacklist(Logout support)
		if rdb.Exists(ctx, "bl:"+tokenStr).Val() == 1 {
			http.Error(w, "Token revoked", http.StatusUnauthorized)
			return
		}
		//Parse JWT
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return jwtSecret, nil
		})
		//Validate token
		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		//Store user info in request context
		ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "email", claims.Email)
		//Continue request
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

/*========== AUTH ==========*/
//Login endpoint
func login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	//Decode request body
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	var userID int
	var hash string
	//Fetch user credentials
	err = db.QueryRow(
		"SELECT id,password FROM users WHERE email=?", req.Email,
	).Scan(&userID, &hash)
	//Validate password
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	//Generate JWT
	token, ttl, err := generateJWT(userID, req.Email)
	if err != nil {
		http.Error(w, "Token generation failed", http.StatusInternalServerError)
		return
	}
	//Send response
	err = json.NewEncoder(w).Encode(map[string]string{
		"access_token": token,
		"expires_in":   ttl.String(),
	})
}

// Logout endpoint(JWT revocation)
func logout(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.Header.Get("Authorization"), " ")
	if len(parts) != 2 {
		http.Error(w, "Invalid token", 401)
		return
	}
	token := parts[1]
	//Store token in Redis blacklist
	rdb.Set(ctx, "bl:"+token, "revoked", time.Minute*15)
	w.Write([]byte("Logged out"))
}

/*========== HANDLERS ==========*/
// createDepartment handles POST /departments
// It creates a new department, stores it in DB,
// and invalidates the Redis cache
func createDepartment(w http.ResponseWriter, r *http.Request) {
	//Struct to hold the incoming JSON data
	var dept Department
	//Decode JSON request body into Department struct
	err := json.NewDecoder(r.Body).Decode(&dept)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	//Validate department fields
	if err := departmentValidation(dept); err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}
	//SQL query to insert a new department
	query := `INSERT INTO departments(name) VALUES(?)`
	//Execute query with department name
	result, err := db.Exec(query, dept.Name)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error inserting:%v", err), http.StatusInternalServerError)
		return
	}
	//Get the auto-generated ID from MySQL
	id, err := result.LastInsertId()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	//Assign generated ID back to response object
	dept.ID = int(id)
	//Invalidate Redis cache so next GET fetches fresh data
	rdb.Del(ctx, "departments:all")
	//Set response Header as JSON
	w.Header().Set("Content-Type", "application/json")
	//Send created department as JSON request
	err = json.NewEncoder(w).Encode(dept)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
}

// getDepartments handles GET /departments
// It first checks Redis cache before hitting the database
func getDepartments(w http.ResponseWriter, r *http.Request) {
	//Redis cache key
	cacheKey := "departments:all"
	//Try fetching departments from redis
	val, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		//Cache hit:return cached data
		log.Println("Cache hit...")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(val))
		return
	}
	//Cache miss: fetch data from DB
	fmt.Println("Missing cache, quering DB...")
	//Execute SELECT query
	result, err := db.Query("SELECT * FROM departments")
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	defer result.Close()
	//Slice to store department records
	var departments []Department
	//Iterate through query results
	for result.Next() {
		var d Department
		//Scan DB row into struct fields
		result.Scan(&d.ID, &d.Name)
		//Append department to slice
		departments = append(departments, d)
	}
	//Convert departments slice to JSON
	data, err := json.Marshal(departments)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	//Store result in Redis with TTL of 10 minutes
	rdb.Set(ctx, cacheKey, data, 10*time.Minute)
	//Return response to client
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// createEmployee handles POST /employees
// It validates input, inserts employee into DB,
// and clears relevant Redis cache
func createEmployee(w http.ResponseWriter, r *http.Request) {
	//Struct to hold incoming JSON payload
	var emp Employee
	//Decode request body into employee struct
	err := json.NewDecoder(r.Body).Decode(&emp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}
	//Validate employee fields
	if err := employeeValidation(emp); err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}
	//SQL insert query
	query := `INSERT INTO employees(name,email,phone,salary,department_id,status) VALUES (?,?,?,?,?,?)`
	//Execute insert with employee data
	result, err := db.Exec(query,
		emp.Name, emp.Email,
		emp.Phone,
		emp.Salary, emp.DepartmentID,
		emp.Status)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}

	//Retrieve auto-generated employee ID
	id, err := result.LastInsertId()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	// Assign ID to response struct
	emp.ID = int(id)
	//Invalidate employees list cache
	rdb.Del(ctx, "employees:all")
	//Send JSON response
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(emp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
}

// getEmployees handles GET /employees
// It fetches employee list from Redis if available,
// otherwise queries DB and caches the result
func getEmployees(w http.ResponseWriter, r *http.Request) {
	cacheKey := "employees:all"
	// Try fetching data from Redis cache
	if val, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
		log.Println("Cache hit...")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(val))
		return
	}
	// Cache miss: query database
	fmt.Println("Missing cache, quering DB...")
	rows, err := db.Query("SELECT * FROM employees")
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	// Slice to store employee records
	var employees []Employee
	// Iterate over rows and scan data
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
	// Marshal employees slice into JSON
	data, err := json.Marshal(employees)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	// Cache the response for 10 minutes
	rdb.Set(ctx, cacheKey, data, 10*time.Minute)
	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// getEmployeeByID handles GET /employees/{id}
// It checks Redis cache before querying the database
func getEmployeeByID(w http.ResponseWriter, r *http.Request) {
	// Get employee ID from URL params
	id := mux.Vars(r)["id"]
	cacheKey := fmt.Sprintf("employee:%s", id)
	// Check Redis cache for employee
	if val, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
		log.Println("Cache hit...")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(val))
		return
	}
	// Cache miss: query database
	fmt.Println("Missing cache, quering DB...")
	var e Employee
	query :=
		`SELECT * FROM employees WHERE id=?`
	// Execute query and scan result into struct
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
	// Marshal employee to JSON
	data, err := json.Marshal(e)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	// Cache employee data
	rdb.Set(ctx, cacheKey, data, 10*time.Minute)
	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// updateEmployee handles PUT /employees/{id}
// It updates employee data and clears related cache
func updateEmployee(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	// Struct to hold updated employee data
	var emp Employee
	// Decode request body
	err := json.NewDecoder(r.Body).Decode(&emp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}
	// Validate updated fields
	if err := employeeValidation(emp); err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}
	// SQL update query
	query := `
	UPDATE employees SET 
	name=?,email=?,phone=?,salary=?,department_id=?,status=?
	WHERE id=?`
	// Execute update query
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
	// Invalidate employee caches
	rdb.Del(ctx, "employees:all")
	rdb.Del(ctx, fmt.Sprintf("employee:%s", id))
	// Send success response
	w.Write([]byte("Employee Updated Successfully."))
}

// deleteEmployee handles DELETE /employees/{id}
// It removes employee from DB and clears cache
func deleteEmployee(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	// Execute delete query
	_, err := db.Exec("DELETE FROM employees WHERE id=?", id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	// Clear related Redis cache
	rdb.Del(ctx, "employees:all")
	rdb.Del(ctx, fmt.Sprintf("employee:%s", id))
	// Send confirmation response
	w.Write([]byte("Deleted employee successfully."))
}

// departmentValidation validates department input fields
func departmentValidation(d Department) error {
	// Check if department name is empty or contains only spaces
	if strings.TrimSpace(d.Name) == "" {
		return errors.New("Department name cannot be Empty.")
	}
	// Ensure department name has at least 2 characters
	if len(d.Name) < 2 {
		return errors.New("Department name cannot be less than 2 Characters")
	}
	// All validations passed
	return nil
}

// employeeValidation validates employee input fields
func employeeValidation(e Employee) error {
	// Validate employee name is not empty
	if strings.TrimSpace(e.Name) == "" {
		return errors.New("Employee name cannot be empty")
	}
	// Basic email format check
	if !strings.Contains(e.Email, "@") {
		return errors.New("Email syntax is Invalid")
	}
	// Restrict email domain to gmail.com
	if !strings.HasSuffix(e.Email, "@gmail.com") {
		return errors.New("Invalid Email. Must contain '@gmail.com'")
	}
	// Ensure email has a prefix before @gmail.com
	prefix := strings.TrimSuffix(e.Email, "@gmail.com")
	if prefix == "" {
		return errors.New("Email must contain a prefix to @gmail.com")
	}
	// Validate phone number presence
	if strings.TrimSpace(e.Phone) == "" {
		return errors.New("Phone number is required")
	}
	// Salary must be zero or positive
	if e.Salary < 0 {
		return errors.New("Salary cannot be less than zero")
	}
	// Department ID must be a valid positive number
	if e.DepartmentID < 0 {
		return errors.New("Invalid department ID")
	}
	// Status field (e.g., active/inactive) must not be empty
	if strings.TrimSpace(e.Status) == "" {
		return errors.New("Status cannot be empty")
	}
	//All validations passed
	return nil
}

/*========== MAIN ==========*/
// EmsHandler initializes the application and starts the HTTP server
func EmsHandler() {
	// Load environment variables from .env file into the process
	godotenv.Load()

	// Read JWT secret key from environment variables
	jwtSecret = []byte(os.Getenv("JWT_SECRET"))
	// Fail fast if JWT secret is not configured
	if len(jwtSecret) == 0 {
		log.Fatal("JWT_SECRET missing")
	}

	// Establish MySQL database connection
	connectDB()
	// Establish Redis connection
	connectrRedis()
	// Initialize Gorilla Mux router
	router := mux.NewRouter()

	// Public authentication route (no JWT required)
	router.HandleFunc("/login", login).Methods("POST")

	// Create a subrouter for protected routes
	protected := router.PathPrefix("/").Subrouter()

	// Apply JWT middleware to all protected routes
	protected.Use(jwtMiddleware)

	// Logout route (JWT required)
	protected.HandleFunc("/logout", logout).Methods("POST")

	// Department routes (currently public)
	router.HandleFunc("/departments", createDepartment).Methods("POST")
	router.HandleFunc("/departments", getDepartments).Methods("GET")

	// Employee routes (currently public)
	router.HandleFunc("/employees", createEmployee).Methods("POST")
	router.HandleFunc("/employees", getEmployees).Methods("GET")
	router.HandleFunc("/employees/{id}", getEmployeeByID).Methods("GET")
	router.HandleFunc("/employees/{id}", updateEmployee).Methods("PUT")
	router.HandleFunc("/employees/{id}", deleteEmployee).Methods("DELETE")

	// Log server startup message
	log.Println("Server running on port:8080")

	// Start HTTP server and listen for incoming requests
	http.ListenAndServe(":8080", router)
}
