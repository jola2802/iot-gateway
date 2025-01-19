package webui

import (
	"database/sql"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// showLoginPage displays the login page to the user.
//
// This function is used to handle GET requests to the login page.
// It renders the login.html template with no data.
func showLoginPage(c *gin.Context) {
	// c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	// if err := tmpl.ExecuteTemplate(c.Writer, "login.html", nil); err != nil {
	// 	c.String(http.StatusInternalServerError, "Error rendering template: %v", err)
	// }
	c.HTML(http.StatusOK, "login.html", nil)
}

// performLogin handles the login form submission.
//
// This function is used to handle POST requests to the login page.
// It retrieves the username and password from the form data, checks them against the database,
// and sets a session cookie if the credentials are valid.
//
// Example:
//
//	curl -X POST -F "username=john" -F "password=hello" http://localhost:8080/login
func performLogin(c *gin.Context) {
	// Get the db instance from the gin.Context
	db, exists := c.Get("db")
	if !exists {
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{"error": "Database connection not found"})
		return
	}

	// Type assert db to *sql.DB
	dbConn := db.(*sql.DB)

	username := c.PostForm("username")
	password := c.PostForm("password")

	var storedPassword string
	err := dbConn.QueryRow("SELECT password FROM users WHERE username = ?", username).Scan(&storedPassword)
	if err != nil || storedPassword != password {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{"error": "Invalid credentials"})
		return
	}

	session := sessions.Default(c)
	session.Set("user", username)
	session.Save()
	c.Redirect(http.StatusFound, "/")
}

// logout logs the user out by deleting the session cookie.
//
// This function is used to handle GET requests to the logout page.
// It deletes the user session and redirects the user to the login page.
func logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Delete("user")
	session.Save()
	c.Redirect(http.StatusFound, "/login")
}

// AuthRequired is a middleware that checks if the user is authenticated.
//
// If the user is not authenticated, it redirects them to the login page.
// Otherwise, it calls the next handler in the chain.
func AuthRequired(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get("user")
	if user == nil {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}
	c.Next()
}
