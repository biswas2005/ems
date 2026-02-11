package project

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func Hash() {
	password := "mypassword123"

	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	fmt.Println(string(hash))
}
