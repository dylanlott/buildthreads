package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

//go:embed templates/*
var resources embed.FS

var t = template.Must(template.ParseFS(resources, "templates/*"))

type server struct {
	db *gorm.DB
}

// Build holds info for a general Build. The BuildsResponse creates
// a list of Builds
type Build struct {
	gorm.Model

	Name        string
	Description string
	Owner       string
	Parts       map[string]interface{} `gorm:"serializer:json"`
}

// BuildsResponse hydrates the Builds html template.
type BuildsResponse struct {
	Builds []Build
}

type BuildDetail struct {
	Build Build
}

func main() {
	// Parse flags and environment variables
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// read postgres dsn from env and fallback to dev dsn if none is set in env.
	var dsn string
	dsn = os.Getenv("BUILDTHREAD_PG_DSN")
	if dsn == "" {
		dsn = "host=localhost user=postgres dbname=postgres port=5432 sslmode=disable"
	}

	// attempt to connect with dsn
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to postgres: %s", err)
	}

	// Migrate models
	db.AutoMigrate(&Build{})

	// TODO: separate out into proper package and file
	s := &server{
		db: db,
	}

	r := chi.NewRouter()

	// Register middlewares
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Register Builds Endpoint
	r.Route("/builds", func(r chi.Router) {
		// Build Main List
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			var builds []Build

			result := s.db.Find(&builds)
			if result.Error != nil {
				log.Printf("failed to parse build: %+v", err)
				http.Error(w, "failed to parse build", http.StatusBadRequest)
				return
			}

			buildsRes := BuildsResponse{
				Builds: builds,
			}

			t.ExecuteTemplate(w, "builds.html.tmpl", buildsRes)
		})

		// Build Detail Route
		r.Route("/{buildID}", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				id := chi.URLParam(r, "buildID")
				parsedID, err := strconv.Atoi(id)
				if err != nil {
					log.Printf("failed to parse build: %+v", err)
					http.Error(w, "failed to parse build", http.StatusBadRequest)
					return
				}
				b := Build{}
				b.ID = uint(parsedID)
				result := db.Find(&b)
				if result.Error != nil {
					log.Printf("failed to parse build: %+v", err)
					http.Error(w, "failed to parse build", http.StatusBadRequest)
					return
				}
				details := BuildDetail{
					Build: b,
				}
				log.Printf("buildDetails: %+v", details)
				t.ExecuteTemplate(w, "build_details.html.tmpl", details)
			})
		})
	})

	// health endpoint establishes the health of the server and postgres instance
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		var dbStatus bool

		tx := s.db.Raw("SELECT 1;")
		if tx.Error != nil {
			log.Printf("postgres health check failed: %+v", tx.Error)
			dbStatus = false
		} else {
			dbStatus = true
		}

		data := map[string]interface{}{
			"Server":   true,
			"Database": dbStatus,
		}

		if err := writeJSON(w, data); err != nil {
			http.Error(w, "failed to write json", http.StatusBadRequest)
			return
		}
	})

	log.Println("listening on", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func writeJSON(w http.ResponseWriter, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}
	w.WriteHeader(http.StatusOK)
	w.Write(b)
	return nil
}
