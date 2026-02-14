package project

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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
)

/*========== GLOBALS ==========*/
// Global shared resources
var (
	db        *sql.DB                //MySQL database connection
	rdb       *redis.Client          //Redis client connection
	ctx       = context.Background() //Shared context for Redis ops
	jwtSecret []byte                 //Secret key for signing JWT
)

/*========== DATA MODELS ==========*/
type Users struct {
	ID       int    `json:"id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

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

/*========== HANDLERS ==========*/

// createDepartment handles POST/departments
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
	if err := DepartmentValidation(dept); err != nil {
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

	//Assign generated ID back to response body
	dept.ID = int(id)
	//Invalidate Redis cache so that next GET fetches fresh data
	rdb.Del(ctx, "departments:all")

	//Send created department as JSON request
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(dept)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
}

// getDepartments handles GET/departments
// It first checks Redis cache before hitting the Database
func getDepartments(w http.ResponseWriter, r *http.Request) {
	//Redis cache key
	cacheKey := "departments:all"

	//Try fetching departments from Redis
	val, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		//Cache hit: return cached data
		log.Println("Cache hit...")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(val))
		return
	}
	//Cache miss, fetch data from DB
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
		//Append departments to slice
		departments = append(departments, d)
	}

	//Convert department slice into JSON
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

// createEmployee handles POST/employees
// It validates input, inserts employees into DB,
// and clears relevant Redis cache
func createEmployee(w http.ResponseWriter, r *http.Request) {
	//Struct to hold incoming JSON payload
	var emp Employee

	//Decode incoming request body into Employee struct
	err := json.NewDecoder(r.Body).Decode(&emp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}

	//Validate employee fields
	if err := EmployeeValidation(emp); err != nil {
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
	//Assign ID to response struct
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

// getEmployees handles GET/employees
// It fetches employees list from Redis if available,
// otherwise queries DB and cache the result
func getEmployees(w http.ResponseWriter, r *http.Request) {
	cacheKey := "employees:all"

	//Try fetching data from Redis cache
	if val, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
		log.Println("Cache hit...")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(val))
		return
	}

	//Cache miss: try fetching from database
	fmt.Println("Missing cache, quering DB...")
	rows, err := db.Query("SELECT * FROM employees")
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	//Slice to store employees record
	var employees []Employee

	//Iterate over rows and scan data
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

	//Marshal employees slice into JSON
	data, err := json.Marshal(employees)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	//Cache the response for 10 minutes
	rdb.Set(ctx, cacheKey, data, 10*time.Minute)
	//Return response
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// getEmployeeByID handles GET/employees/{id}
// It checks Redis cache, before quering the DB
func getEmployeeByID(w http.ResponseWriter, r *http.Request) {

	//Get employee id from URL params
	id := mux.Vars(r)["id"]
	cacheKey := fmt.Sprintf("employee:%s", id)
	//Check Redis cache for employee
	if val, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
		log.Println("Cache hit...")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(val))
		return
	}

	//Cache miss: query database
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

	//Marshal employee to JSON
	data, err := json.Marshal(e)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	//Cache employee data
	rdb.Set(ctx, cacheKey, data, 10*time.Minute)
	//Return response
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// updateEmployee handles PUT/employees/{id}
// Updates an existing employee
func updateEmployee(w http.ResponseWriter, r *http.Request) {
	//Extract the "id" parameter from the request URL using mux
	id := mux.Vars(r)["id"]
	//Declare an employee struct to hold the decoded JSON data
	var emp Employee

	//Decode the request body JSON into the employee struct
	err := json.NewDecoder(r.Body).Decode(&emp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}
	//Validate the employee data
	if err := EmployeeValidation(emp); err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}

	//sql query to update employee details to the database
	query := `
	UPDATE employees SET 
	name=?,email=?,phone=?,salary=?,department_id=?,status=?
	WHERE id=?`

	//Execute the SQL update query with the provided employee data
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
	//If execution fails return an error
	if err1 != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err1), http.StatusInternalServerError)
		return
	}

	//Clear Redis cache for all employees and the specific updated employee
	rdb.Del(ctx, "employees:all")
	rdb.Del(ctx, fmt.Sprintf("employee:%s", id))
	//Send a success response back to the client
	w.Write([]byte("Employee Updated Successfully."))
}

// deleteEmployee handles DELETE/employees/{id}
// Removes an employee from the database
func deleteEmployee(w http.ResponseWriter, r *http.Request) {
	// Extract the "id" parameter from the request URL using mux
	id := mux.Vars(r)["id"]

	// Execute SQL query to delete the employee record by ID
	_, err := db.Exec("DELETE FROM employees WHERE id=?", id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}

	// Clear Redis cache for all employees and the specific deleted employee
	rdb.Del(ctx, "employees:all")
	rdb.Del(ctx, fmt.Sprintf("employee:%s", id))
	// Send a success response back to the client
	w.Write([]byte("Deleted employee successfully."))
}

/*========== MAIN ==========*/
func EmsHandler() {

	// Load environment variables from .env file
	godotenv.Load()

	// Read JWT secret from environment variables
	jwtSecret = []byte(os.Getenv("JWT_SECRET"))

	// Ensure JWT secret is present, otherwise terminate the program
	if len(jwtSecret) == 0 {
		log.Fatal("JWT_SECRET missing")
	}

	// Connection to the database
	ConnectDB()
	// Connection to Redis
	ConnectRedis()
	// Create a new Gorilla mux router
	router := mux.NewRouter()

	// Public route for login
	router.HandleFunc("/login", Login).Methods("POST")
	// Create a subrouter for protected routes
	protected := router.PathPrefix("/").Subrouter()
	// Apply JwtMiddleware to protected routes
	protected.Use(JwtMiddleware)

	// Protected route for logout
	protected.HandleFunc("/logout", Logout).Methods("POST")

	// Department routes (protected)
	protected.HandleFunc("/departments", createDepartment).Methods("POST")
	protected.HandleFunc("/departments", getDepartments).Methods("GET")

	// Employee routes (protected)
	protected.HandleFunc("/employees", createEmployee).Methods("POST")
	protected.HandleFunc("/employees", getEmployees).Methods("GET")
	protected.HandleFunc("/employees/{id}", getEmployeeByID).Methods("GET")
	protected.HandleFunc("/employees/{id}", updateEmployee).Methods("PUT")
	protected.HandleFunc("/employees/{id}", deleteEmployee).Methods("DELETE")

	//Log server startup message
	log.Println("Server running on port:8080")
	// Start the HTTP server on port 8080
	http.ListenAndServe(":8080", router)
}
