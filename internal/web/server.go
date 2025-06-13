package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"dsdeathcounter/internal/counter"
)

//go:embed templates/* static/*
var content embed.FS

// Server serves the death counter web interface
type Server struct {
    counterService *counter.Service
    addr           string
    templates      *template.Template
}

// NewServer creates a new web server
func NewServer(counterService *counter.Service, addr string) *Server {
    // Parse templates from embedded filesystem
    templates := template.Must(template.ParseFS(content, "templates/*.html"))
    
    return &Server{
        counterService: counterService,
        addr:           addr,
        templates:      templates,
    }
}

// Start begins serving the web interface
func (s *Server) Start() error {
    // Setup static file serving
    staticFS, err := fs.Sub(content, "static")
    if err != nil {
        return err
    }
    http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
    
    // Setup root handler
    http.HandleFunc("/", s.handleRoot)
    
    return http.ListenAndServe(s.addr, nil)
}

// getTemplateForGame returns the appropriate template name for the current game
func (s *Server) getTemplateForGame(game string) string {
    // For no game detection, specifically return the nogame template
    if game == "No game detected" {
        return "nogame.html"
    }
    
    // For games, remove spaces and convert to lowercase
    templateName := strings.ToLower(strings.ReplaceAll(game, " ", ""))
    
    // Check if the template exists using the correct template name
    if s.templates.Lookup(templateName + ".html") != nil {
        return templateName + ".html"
    }
    
    // Fallback to nogame.html if we can't find a template for this game
    return "nogame.html"
}

// handleRoot handles the root endpoint
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
    deaths, game := s.counterService.GetCurrentState()
    
    // Choose the appropriate template based on the current game
    templateName := s.getTemplateForGame(game)
    
    // Execute the template
    err := s.templates.ExecuteTemplate(w, templateName, map[string]interface{}{
        "Deaths": deaths,
        "Game":   game,
    })
    
    if err != nil {
        http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
    }
}