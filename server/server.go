package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"rxDB/lwdb"
	passwordreset "rxDB/passreset"
	"strings"
	"time"
)

type TemplateData struct {
	Keys     []string
	Value    string
	Error    string
	Stage    string
	Username string
}

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

	var sb strings.Builder
	sb.WriteString(username[:2])
	sb.WriteString(strings.Repeat("*", len(username)-4))
	sb.WriteString(username[len(username)-2:])
	sb.WriteString("@")
	sb.WriteString(domain)

	return sb.String()
}

func resetPassword(username string, db *lwdb.SimpleDB) (string, error) {
	newPassword := "new-password-" + username
	err := db.Set(username, newPassword)
	if err != nil {
		return "", fmt.Errorf("failed to save new password: %w", err)
	}
	return newPassword, nil
}

func main() {
	dataPath := filepath.Join("..", "lwdb", "dbData", "data.txt")
	db := lwdb.NewSimpleDB(dataPath)

	templatePath := filepath.Join("statics", "html", "test.html")
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Fatalf("Parse template err: %v", err)
		return
	}

	codeStore := passwordreset.NewCodeStore()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		dataMap := db.GetData()
		keys := make([]string, 0, len(dataMap))
		for k := range dataMap {
			keys = append(keys, k)
		}
		data := TemplateData{Keys: keys, Value: "", Stage: "initial"}
		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, fmt.Sprintf("Template err: %v", err), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "Missing key", http.StatusBadRequest)
			return
		}
		value, ok := db.Get(key)
		if !ok {
			dataMap := db.GetData()
			keys := make([]string, 0, len(dataMap))
			for k := range dataMap {
				keys = append(keys, k)
			}
			data := TemplateData{Keys: keys, Value: "", Error: "Username not found", Stage: "initial"}
			if err := tmpl.Execute(w, data); err != nil {
				http.Error(w, fmt.Sprintf("Template err: %v", err), http.StatusInternalServerError)
				return
			}
			return
		}

		code, err := codeStore.GenerateCode(value, 10*time.Minute)
		if err != nil {
			http.Error(w, "Gen code err", http.StatusInternalServerError)
			return
		}

		fmt.Printf("Code: %s for %s\n", code, key)

		dataMap := db.GetData()
		keys := make([]string, 0, len(dataMap))
		for k := range dataMap {
			keys = append(keys, k)
		}
		data := TemplateData{Keys: keys, Value: maskEmail(value), Stage: "verify", Username: key}
		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, fmt.Sprintf("Template err: %v", err), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		username := r.FormValue("username")
		code := r.FormValue("code")

		email, ok := db.Get(username)
		if !ok {
			http.Error(w, "User not found", http.StatusBadRequest)
			return
		}

		if !codeStore.ValidateCode(code, email) {
			dataMap := db.GetData()
			keys := make([]string, 0, len(dataMap))
			for k := range dataMap {
				keys = append(keys, k)
			}
			data := TemplateData{Keys: keys, Value: "", Error: "Invalid code", Stage: "verify", Username: username}
			if err := tmpl.Execute(w, data); err != nil {
				http.Error(w, fmt.Sprintf("Template err: %v", err), http.StatusInternalServerError)
				return
			}
			return
		}

		newPassword, err := resetPassword(username, db)
		if err != nil {
			log.Printf("Reset err: %v", err)
			dataMap := db.GetData()
			keys := make([]string, 0, len(dataMap))
			for k := range dataMap {
				keys = append(keys, k)
			}
			data := TemplateData{Keys: keys, Value: "", Error: "Reset failed", Stage: "verify", Username: username}
			if err := tmpl.Execute(w, data); err != nil {
				http.Error(w, fmt.Sprintf("Template err: %v", err), http.StatusInternalServerError)
				return
			}
			return
		}

		dataMap := db.GetData()
		keys := make([]string, 0, len(dataMap))
		for k := range dataMap {
			keys = append(keys, k)
		}
		data := TemplateData{Keys: keys, Value: "Password reset", Stage: "reset"}
		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, fmt.Sprintf("Template err: %v", err), http.StatusInternalServerError)
			return
		}

		fmt.Printf("New Pass: %s\n", newPassword)
	})

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("statics"))))

	fmt.Println("Server :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server err: %v", err)
	}
}
