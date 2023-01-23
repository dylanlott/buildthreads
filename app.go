package main

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

//go:embed templates/*
var resources embed.FS

var t = template.Must(template.ParseFS(resources, "templates/*"))

type server struct {
	db *gorm.DB
}

// Build holds info for a general Build
type Build struct {
	gorm.Model

	Name        string
	Description string
	Owner       string
	Parts       map[string]interface{} `gorm:"serializer:json"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dsn := "host=localhost user=postgres dbname=postgres port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to postgres: %s", err)
	}

	db.AutoMigrate(&Build{})

	s := &server{
		db: db,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data := map[string]string{
			"Region": os.Getenv("FLY_REGION"),
		}
		t.ExecuteTemplate(w, "index.html.tmpl", data)
	})

	http.HandleFunc("/builds", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			data := map[string]string{
				"Builds": "", // TODO fill this with query data
			}
			t.ExecuteTemplate(w, "builds.html.tmpl", data)
		case http.MethodPost:
			defer r.Body.Close()
			b, err := ioutil.ReadAll(r.Body)
			if err != nil {
				log.Printf("failed to parse build: %+v", err)
				http.Error(w, "failed to parse build", http.StatusBadRequest)
				return
			}
			build := &Build{}
			err = json.Unmarshal(b, &build)
			if err != nil {
				log.Printf("failed to parse build: %+v", err)
				http.Error(w, "failed to parse build", http.StatusBadRequest)
				return
			}

			result := s.db.Create(&build)
			if result.Error != nil {
				log.Printf("failed to create build: %+v", err)
				http.Error(w, "failed to parse build", http.StatusBadRequest)
				return
			}

			log.Printf("result: %+v", result)

			w.WriteHeader(http.StatusOK)
			return
		case http.MethodPut:
		case http.MethodDelete:
		default:
			panic("not impl")
		}
	})

	log.Println("listening on", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
