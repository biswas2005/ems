package project

import (
	"errors"
	"strings"
)

func DepartmentValidation(d Department) error {

	if strings.TrimSpace(d.Name) == "" {
		return errors.New("Department name cannot be Empty.")
	}

	if len(d.Name) < 2 {
		return errors.New("Department name cannot be less than 2 Characters")
	}

	return nil
}

func EmployeeValidation(e Employee) error {

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
