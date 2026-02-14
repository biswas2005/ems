# EMPLOYEE MANAGEMENT SYSTEM (EMS)  
A secure **Employee Management System API** built with GO, using:  
* JWT Authentication (Access + Refresh tokens)  
* Cookie-based authentication  
* MySQL database  
* Redis caching  
* Gorilla Mux router  
* Validation layer  
* Middleware-based security  
***
## Features  
* Admin Login (JWT-based)  
* Access + Refresh token flow  
* Secure HttpOnly cookies  
* Redis token blacklist (logout invalidation)  
* Redis caching for GET endpoints  
* CRUD for Departments  
* CRUD for Employees  
* Middleware-based route protection  
* Environment-based configuration
***
## Tech Stack  
| Technology | Purpose |
|-------------|----------------|
| GO | Backend API |
| MySQL | Persistent Database |
| Redis | Caching & token storage |
| Gorilla Mux | HTTP routing |
| JWT (golang-jwt) | Authentication |
| godotenv | Environment config |
***
## Project Structure  
```Go 
project/
│
├── auth.go
├── database.go
├── jwt.go
├── handlers.go
├── validation.go
└── main.go 
```
*** 
## Authentication Flow  
**1. Login**  
* Admin credentials are stored in environment variables  
* >On successful login :    
* Access token (15 min) → stored in `access_token` cookie  
* Refresh token (7 days) → stored in refresh_token cookie  
* Refresh token stored in Redis  

**2. Access Protected Routes**
* >JWT Middleware  
* Reads `access_token` from cookie  
* Checks Redis blacklist
* Validates JWT signature  
* Injects user info into request context  

**3. Refresh Token**  
* Uses refresh_token cookie  
* Validates Redis record  
* Issues new access token

**4. Logout**  
* Blacklists access token in Redis  
* Deletes refresh token from Redis  
* Expires both cookies
***
## API Endpoints  
### Public
**POST`/login`**  
Login as admin.  
```json
{
  "email":"abcd@gmail.com",
  "password":"abcd123"
}
```
### Protected Routes  
**POST`/logout`**  
Logout and invalidate tokens. 
 
---
### Department APIs  
**POST`/departments`**  
Create department.  
```json
{
  "name":"HR"
}
```
**GET`/departments`**  
Get all departments (cached in Redis for 10 minutes).

---
### Employee APIs  
**POST`/employees`**  
Create employee  
```json
{
  "name": "Abhi",
  "email": "abhi@gmail.com",
  "phone": "9876543210",
  "salary": 50000,
  "department_id": 1,
  "status": "active"
}
```
**GET`/employees`**  
Get all employees (cached).  

**GET`/employees/{id}`**  
Get employee by id (cached). 
 
**PUT`/employees/{id}`**  
Update employee.  

**DELETE`/employee/{id}`**  
Delete employee.  
***
## Validation Rules  
### Department  
* Name cannot be empty.  
* Minimum 2 characters.  
### Employee  
* Name required  
* Email must:  
  * Contain `@`  
  * End with `@gmail.com`  
* Salary cannot be negative  
* Department ID must be valid  
* Status required
***
## Environment Variables  
Create a `.env` file:  
```ini
JWY_SECRET=place_something  

ADMIN_EMAIL=admin@gmail.com  
ADMIN_PASS=admin  

MySQL_DSN=user:password@tcp(127.0.0.1:3306)/ems
```
***
## Database Setup  
### Departments table  
```sql
CREATE TABLE departments (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL
);
```
### Employees table  
```sql
CREATE TABLE employees (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(100),
    phone VARCHAR(20),
    salary DOUBLE,
    department_id INT,
    status VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```
***
## Redis Usage  
Redis is used for:  
* Caching departments list  
* Caching employees list  
* Caching employee by ID  
* Storing refresh tokens  
* Blacklisting access tokens
***
## Running the Project  
**1. Install Dependencies**  
```bash
go mod tidy
```
**2. Start MySQL and Redis**  
Make sure:  
```nginx
MySQL → localhost:3306
Redis → localhost:6379  
```
**3. Run Server**  
```bash
go run main.go
```
Server starts on:  
```arduino
http://localhost:8080
```
***
## Security Notes  
* Uses HttpOnly cookies  
* Access tokens expire in 15 minutes  
* Refresh tokens expire in 7 days  
* Logout invalidates tokens via Redis blacklist  
* Middleware prevents unauthorized access  
***
## Future Improvements  
* Use bcrypt instead of plain admin password  
* Add role-based access  
* Add pagination  
* Add Swagger documentation  
* Use Docker for deployment  
* Add unit tests  
* Use HTTPS in production  
***
## Thanks  
Thank you for visiting the Employee Management System repository. Feel free to reach out if you want help setting up, extending, or deploying this project!

***
## Author 
Abhi Biswas