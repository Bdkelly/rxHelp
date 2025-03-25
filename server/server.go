// server/main.go
package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"rxDB/lwdb"
	passwordreset "rxDB/passreset" // Import your new passwordreset module
	"strings"

	//"sync"
	"time"
)

// Define a struct to hold data for our HTML template.
type TemplateData struct {
	Keys     []string
	Value    string
	Error    string
	Stage    string
	Username string
}

// maskEmail partially masks an email address.
func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "Invalid Email"
	}
	username := parts[0]
	domain := parts[1]
	if len(username) <= 3 {
		return username + "@" + domain
	}
	maskedUsername := username[:2] + strings.Repeat("*", len(username)-4) + username[len(username)-2:]
	return maskedUsername + "@" + domain
}

// resetPassword resets the user's password and sends a new password.
func resetPassword(username string, db *lwdb.SimpleDB) (string, error) {
	// In a real application, you would generate a cryptographically secure password.
	newPassword := "new-password-" + username // Placeholder
	err := db.Set(username, newPassword)
	if err != nil {
		return "", fmt.Errorf("failed to save new password: %w", err)
	}
	return newPassword, nil
}

func main() {
	// Construct the path to data.txt
	dataPath := filepath.Join("..", "lwdb", "dbData", "data.txt")

	// Initialize the database using the lwdb package.
	db := lwdb.NewSimpleDB(dataPath)

	// --- HTML Template ---
	// Parse the HTML template file.
	templatePath := filepath.Join("statics", "html", "test.html")
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Fatalf("Error parsing HTML template: %v", err)
		return
	}

	// --- HTTP Handlers ---
	// Session management (in-memory for simplicity, use a proper store in production)
	//var sessionLock sync.Mutex

	//Initialize the code store
	codeStore := passwordreset.NewCodeStore()

	// Handler for the root path ("/"). This will display the initial page.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Get all keys from the database.
		dataMap := db.GetData()

		var keys []string
		for k := range dataMap {
			keys = append(keys, k)
		}

		// Create the data to pass to the template.
		data := TemplateData{Keys: keys, Value: "", Stage: "initial"}

		// Execute the template, passing in the data.
		err = tmpl.Execute(w, data)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error executing template: %v", err), http.StatusInternalServerError)
			return
		}
	})

	// Handler for the /get path.  This handles the initial submission of the username.
	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "Missing key", http.StatusBadRequest)
			return
		}
		value, ok := db.Get(key)
		if !ok {
			dataMap := db.GetData()
			var keys []string
			for k := range dataMap {
				keys = append(keys, k)
			}
			data := TemplateData{Keys: keys, Value: "", Error: "Username not found", Stage: "initial"}
			err = tmpl.Execute(w, data)
			if err != nil {
				http.Error(w, fmt.Sprintf("Error executing template: %v", err), http.StatusInternalServerError)
				return
			}
			return
		}

		// Generate and store the passcode.
		code, err := codeStore.GenerateCode(value, 10*time.Minute) //value contains the email
		if err != nil {
			http.Error(w, "Failed to generate code", http.StatusInternalServerError)
			return
		}

		fmt.Printf("Passcode for user %s: %s\n", key, code) // Print passcode

		// Transition to the verification stage.
		dataMap := db.GetData()
		var keys []string
		for k := range dataMap {
			keys = append(keys, k)
		}
		data := TemplateData{Keys: keys, Value: maskEmail(value), Stage: "verify", Username: key} //show masked email and the username
		err = tmpl.Execute(w, data)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error executing template: %v", err), http.StatusInternalServerError)
			return
		}
	})

	// Handler for the /verify path.  This handles the submission of the verification code.
	http.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		username := r.FormValue("username")
		code := r.FormValue("code")

		email, ok := db.Get(username) //get the email from the database.
		if !ok {
			http.Error(w, "Username not found", http.StatusBadRequest)
			return
		}

		if !codeStore.ValidateCode(code, email) {
			dataMap := db.GetData()
			var keys []string
			for k := range dataMap {
				keys = append(keys, k)
			}
			data := TemplateData{Keys: keys, Value: "", Error: "Invalid verification code.", Stage: "verify", Username: username}
			err = tmpl.Execute(w, data)
			if err != nil {
				http.Error(w, fmt.Sprintf("Error executing template: %v", err), http.StatusInternalServerError)
				return
			}
			return
		}

		// Code is valid, reset password.
		newPassword, err := resetPassword(username, db)
		if err != nil {
			log.Printf("Error resetting password: %v", err)
			dataMap := db.GetData()
			var keys []string
			for k := range dataMap {
				keys = append(keys, k)
			}
			data := TemplateData{Keys: keys, Value: "", Error: "Failed to reset password. Please try again.", Stage: "verify", Username: username}
			err = tmpl.Execute(w, data)
			if err != nil {
				http.Error(w, fmt.Sprintf("Error executing template: %v", err), http.StatusInternalServerError)
				return
			}
			return
		}

		dataMap := db.GetData()
		var keys []string
		for k := range dataMap {
			keys = append(keys, k)
		}
		data := TemplateData{Keys: keys, Value: "Password has been reset.  Check console for new password.", Stage: "reset"}
		err = tmpl.Execute(w, data)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error executing template: %v", err), http.StatusInternalServerError)
			return
		}

		fmt.Printf("New Password: %s\n", newPassword)
	})

	// --- Serve Static Files (CSS) ---
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("statics"))))

	// --- Start the Web Server ---
	fmt.Println("Web server listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Error starting web server: %v", err)
	}
}
