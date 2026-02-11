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
	//Password hashing
)

/*========== GLOBALS ==========*/
// Global shared resources
var (
	db        *sql.DB
	rdb       *redis.Client
	ctx       = context.Background()
	jwtSecret []byte
)

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

func createDepartment(w http.ResponseWriter, r *http.Request) {

	var dept Department

	err := json.NewDecoder(r.Body).Decode(&dept)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := DepartmentValidation(dept); err != nil {
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

	if err := EmployeeValidation(emp); err != nil {
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

	if err := EmployeeValidation(emp); err != nil {
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

func EmsHandler() {

	godotenv.Load()

	jwtSecret = []byte(os.Getenv("JWT_SECRET"))

	if len(jwtSecret) == 0 {
		log.Fatal("JWT_SECRET missing")
	}

	ConnectDB()

	ConnectRedis()

	router := mux.NewRouter()

	router.HandleFunc("/register", Register).Methods("POST")

	router.HandleFunc("/login", Login).Methods("POST")

	protected := router.PathPrefix("/").Subrouter()

	protected.Use(JwtMiddleware)

	protected.HandleFunc("/logout", Logout).Methods("POST")

	protected.HandleFunc("/departments", createDepartment).Methods("POST")
	protected.HandleFunc("/departments", getDepartments).Methods("GET")

	protected.HandleFunc("/employees", createEmployee).Methods("POST")
	protected.HandleFunc("/employees", getEmployees).Methods("GET")
	protected.HandleFunc("/employees/{id}", getEmployeeByID).Methods("GET")
	protected.HandleFunc("/employees/{id}", updateEmployee).Methods("PUT")
	protected.HandleFunc("/employees/{id}", deleteEmployee).Methods("DELETE")

	log.Println("Server running on port:8080")

	http.ListenAndServe(":8080", router)
}
