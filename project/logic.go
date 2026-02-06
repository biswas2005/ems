package project

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
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
	Salary       string     `json:"salary"`
	DepartmentID string     `json:"department_id"`
	Status       bool       `json:"status"`
	CreatedAt    *time.Time `json:"created_at"`
}

func createDepartment(w http.ResponseWriter, r *http.Request) {
	var dept Department
	err := json.NewDecoder(r.Body).Decode(&dept)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
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
		http.Error(w, fmt.Sprintf("Error:%v",err), http.StatusBadRequest)
		return
	}

	query := `INSERT INTO employees(name,email,phone,salary,department_id,status) VALUES (?,?,?,?,?,?)`
	result, err := db.Exec(query,
		emp.Name, emp.Email,
		emp.Phone, emp.DepartmentID,
		emp.Status)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v",err), http.StatusInternalServerError)
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error:%v",err), http.StatusInternalServerError)
		return
	}
	emp.ID = int(id)

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(emp)
	if err != nil {
		http.Error(w, fmt.Sprintln(err), http.StatusInternalServerError)
		return
	}
}

func getEmployees(w http.ResponseWriter, r *http.Request) {

	rows, err := db.Query("SELECT * FROM employees")
	if err != nil {
		http.Error(w, fmt.Sprintln(err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var employees []Employee
	for rows.Next() {
		var e Employee
		rows.Scan(
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
		http.Error(w, fmt.Sprintln(err), http.StatusInternalServerError)
		return
	}
}

func getEmployeeByID(w http.ResponseWriter, r *http.Request) {

	id := mux.Vars(r)["id"]
	var e Employee
	query :=
		`SELECT * FROM employees WHERE id=?`
	err := db.QueryRow(query, id).Scan(
		&e.Name,
		&e.Email,
		&e.Phone, &e.Salary,
		&e.DepartmentID,
		&e.Status,
		&e.CreatedAt)
	if err != nil {
		http.Error(w, fmt.Sprintln(err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(e)
	if err != nil {
		http.Error(w, fmt.Sprintln(err), http.StatusInternalServerError)
		return
	}
}
