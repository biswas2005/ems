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
	//Gorilla mux router
	"github.com/gorilla/mux"
	//load environment variables from .env
	"github.com/joho/godotenv"
	//Redis client
	"github.com/redis/go-redis/v9"
)

// Global database connection
var db *sql.DB

// Global redis client
var rdb *redis.Client

// Global context used by Redis operations
var ctx = context.Background()

// Connect to Redis server
func connectrRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", //redis address
		Password: "",               //No password
		DB:       0,                //Default DB
	})

	//Check redis connectivity
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis:%v", err)
	}
	log.Println("Redis connected successfully.")
}

// Department model
type Department struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Employee model
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

// Connect to MySQL database
func connectDB() {
	var err error

	//Read DSN from environment variable
	dsn := os.Getenv("MySQL_DSN")
	if dsn == "" {
		log.Fatal("MySQL_DSN not set in environment")
	}

	//Open database connection
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}

	//Verify database connection
	err = db.Ping()
	if err != nil {
		log.Fatal("Database not reachable", err)
	}
	log.Println("Database connected successfully.")
}

// Create a new department
func createDepartment(w http.ResponseWriter, r *http.Request) {
	var dept Department
	//Decode request body into struct
	err := json.NewDecoder(r.Body).Decode(&dept)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	//Validate department input
	if err := departmentValidation(dept); err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}

	//Insert department into database
	query := `INSERT INTO departments(name) VALUES(?)`
	result, err := db.Exec(query, dept.Name)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error inserting:%v", err), http.StatusInternalServerError)
		return
	}

	//Get auto-generated ID
	id, err := result.LastInsertId()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	dept.ID = int(id)

	//Invalidate department cache
	rdb.Del(ctx, "departments:all")

	//Send response
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(dept)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
}

// Fetch all departments (with Redis caching)
func getDepartments(w http.ResponseWriter, r *http.Request) {
	cacheKey := "departments:all"

	//Try fetching from Redis cache
	val, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		log.Println("Cache hit...")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(val))
		return
	}
	//Cache miss --> query database
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

	//Cache the result
	data, err := json.Marshal(departments)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	rdb.Set(ctx, cacheKey, data, 10*time.Minute)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// Create a new employee
func createEmployee(w http.ResponseWriter, r *http.Request) {

	var emp Employee
	//Decode JSON request
	err := json.NewDecoder(r.Body).Decode(&emp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}

	//Validate employee input
	if err := employeeValidation(emp); err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}

	//Insert employee into database
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

	//Get inserted employee ID
	id, err := result.LastInsertId()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	emp.ID = int(id)

	//Invalidate employees cache
	rdb.Del(ctx, "employees:all")

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(emp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
}

// Fetch all employees with Redis caching
func getEmployees(w http.ResponseWriter, r *http.Request) {
	cacheKey := "employees:all"

	//Check Redis cache
	if val, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
		log.Println("Cache hit...")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(val))
		return
	}
	//Cache miss --> Query database
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

	//Cache employees list
	data, err := json.Marshal(employees)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	rdb.Set(ctx, cacheKey, data, 10*time.Minute)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// Fetch employees by ID (with Redis caching)
func getEmployeeByID(w http.ResponseWriter, r *http.Request) {

	id := mux.Vars(r)["id"]
	cacheKey := fmt.Sprintf("employee:%s", id)

	//Try redis first
	if val, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
		log.Println("Cache hit...")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(val))
		return
	}
	//Cache miss --> Query database
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
	//Cache single employee
	data, err := json.Marshal(e)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
	rdb.Set(ctx, cacheKey, data, 10*time.Minute)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// Update employee details
func updateEmployee(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var emp Employee
	//Decode request body
	err := json.NewDecoder(r.Body).Decode(&emp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}

	//Validate input
	if err := employeeValidation(emp); err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusBadRequest)
		return
	}

	//Update employee record
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

	//Invalidate related cache
	rdb.Del(ctx, "employees:all")
	rdb.Del(ctx, fmt.Sprintf("employee:%s", id))
	w.Write([]byte("Employee Updated Successfully."))
}

// Delete employee
func deleteEmployee(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	//Delete from database
	_, err := db.Exec("DELETE FROM employees WHERE id=?", id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}

	//Clear cache
	rdb.Del(ctx, "employees:all")
	rdb.Del(ctx, fmt.Sprintf("employee:%s", id))
	w.Write([]byte("Deleted employee successfully."))
}

// Validate department input
func departmentValidation(d Department) error {

	if strings.TrimSpace(d.Name) == "" {
		return errors.New("Department name cannot be Empty.")
	}
	if len(d.Name) < 2 {
		return errors.New("Department name cannot be less than 2 Characters")
	}
	return nil
}

// Validate employee input
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

// Application entry handler
func EmsHandler() {
	//Load environment variables
	godotenv.Load()

	//Connect database and redis
	connectDB()
	connectrRedis()

	//Initialize router
	router := mux.NewRouter()

	//Department routes
	router.HandleFunc("/departments", createDepartment).Methods("POST")
	router.HandleFunc("/departments", getDepartments).Methods("GET")

	//Employee routes
	router.HandleFunc("/employees", createEmployee).Methods("POST")
	router.HandleFunc("/employees", getEmployees).Methods("GET")
	router.HandleFunc("/employees/{id}", getEmployeeByID).Methods("GET")
	router.HandleFunc("/employees/{id}", updateEmployee).Methods("PUT")
	router.HandleFunc("/employees/{id}", deleteEmployee).Methods("DELETE")

	log.Println("Server running on port:8080")
	http.ListenAndServe(":8080", router)
}
