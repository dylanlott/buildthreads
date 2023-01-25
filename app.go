package main

import (
	"embed"
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
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// TODO: read DSN from env vars
	dsn := "host=localhost user=postgres dbname=postgres port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to postgres: %s", err)
	}

	db.AutoMigrate(&Build{})

	// TODO: separate out into proper package and file
	s := &server{
		db: db,
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		data := map[string]string{
			"Region": os.Getenv("FLY_REGION"),
		}
		t.ExecuteTemplate(w, "index.html.tmpl", data)
	})

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

	log.Println("listening on", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
