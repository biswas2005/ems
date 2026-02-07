package project

import (
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
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

var db *sql.DB

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

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(dept)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
}

func getDepartments(w http.ResponseWriter, r *http.Request) {

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

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(departments)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
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

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(emp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
}

func getEmployees(w http.ResponseWriter, r *http.Request) {

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

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(employees)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
}

func getEmployeeByID(w http.ResponseWriter, r *http.Request) {

	id := mux.Vars(r)["id"]
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
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(e)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}
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
	w.Write([]byte("Employee Updated Successfully."))
}

func deleteEmployee(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	_, err := db.Exec("DELETE FROM employees WHERE id=?", id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v", err), http.StatusInternalServerError)
		return
	}

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

func EmsHandler() {
	godotenv.Load()

	connectDB()

	router := mux.NewRouter()

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
