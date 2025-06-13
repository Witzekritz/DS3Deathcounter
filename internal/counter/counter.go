package counter

import (
	"log"
	"sync"
	"time"

	"dsdeathcounter/internal/games"

	"golang.org/x/sys/windows"
)

// Service manages the death counter state
type Service struct {
    mu             sync.RWMutex
    currentDeaths  int32
    currentGame    string
    updateInterval time.Duration
}

// NewService creates a new counter service
func NewService() *Service {
    return &Service{
        currentGame:    "No game detected",
        updateInterval: 500 * time.Millisecond,
    }
}

// Start begins monitoring for game processes and death counts
func (s *Service) Start() {
    go s.monitorGames()
}

// GetCurrentState returns the current death count and game
func (s *Service) GetCurrentState() (int32, string) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.currentDeaths, s.currentGame
}

// monitorGames continuously checks for game processes and death counts
func (s *Service) monitorGames() {
    for {
        process, err := games.FindGameProcess()
        if err != nil {
            s.mu.Lock()
            if s.currentGame != "No game detected" {
                log.Printf("Game process closed or not found: %v", err)
                s.currentGame = "No game detected"
            }
            s.mu.Unlock()
            time.Sleep(2 * time.Second)
            continue
        }

        s.mu.Lock()
        if s.currentGame != process.Game.Name {
            s.currentGame = process.Game.Name
            log.Printf("Found game: %s", s.currentGame)
        }
        s.mu.Unlock()

        deaths, err := games.GetDeathCount(process)
        if err != nil {
            log.Printf("Error reading death count: %v", err)
            windows.CloseHandle(process.Handle)
            time.Sleep(2 * time.Second)
            continue
        }

        s.mu.Lock()
        if deaths != s.currentDeaths {
            s.currentDeaths = deaths
            log.Printf("%s deaths updated: %d", s.currentGame, s.currentDeaths)
        }
        s.mu.Unlock()

        windows.CloseHandle(process.Handle)
        time.Sleep(s.updateInterval)
    }
}