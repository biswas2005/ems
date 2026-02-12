package project

import (
	"errors"
	"strings"
)

// DepartmentValidation ensures that a department object has valid data
func DepartmentValidation(d Department) error {

	//Check if department name is empty or only blank space
	if strings.TrimSpace(d.Name) == "" {
		return errors.New("Department name cannot be Empty.")
	}
	//Ensure department name has atleast 2 characters
	if len(d.Name) < 2 {
		return errors.New("Department name cannot be less than 2 Characters")
	}
	//If all checks pass, return nil (no error)
	return nil
}

// EmployeeValidation ensures that a employee object has valid data
func EmployeeValidation(e Employee) error {

	//Check if employee name is empty or whitespaces only
	if strings.TrimSpace(e.Name) == "" {
		return errors.New("Employee name cannot be empty")
	}
	//Validate email contains '@'
	if !strings.Contains(e.Email, "@") {
		return errors.New("Email syntax is Invalid")
	}
	//Ensure email ends with @gmail.com
	if !strings.HasSuffix(e.Email, "@gmail.com") {
		return errors.New("Invalid Email. Must contain '@gmail.com'")
	}
	//Ensure email has a prefix before @gmail.com
	prefix := strings.TrimSuffix(e.Email, "@gmail.com")
	if prefix == "" {
		return errors.New("Email must contain a prefix to @gmail.com")
	}
	//Check if phone number is provided
	if strings.TrimSpace(e.Phone) == "" {
		return errors.New("Phone number is required")
	}
	//Salary must not be negative
	if e.Salary < 0 {
		return errors.New("Salary cannot be less than zero")
	}
	//Department ID must be non-negative
	if e.DepartmentID < 0 {
		return errors.New("Invalid department ID")
	}
	//Status must not be empty
	if strings.TrimSpace(e.Status) == "" {
		return errors.New("Status cannot be empty")
	}
	//If all checks pass, return nil(no error)
	return nil
}
