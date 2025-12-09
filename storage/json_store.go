package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type UserStore struct {
	mu           sync.RWMutex
	users        map[string]string // discord_id -> dota_account_id
	lastMatches  map[string]int64  // discord_id -> last_match_id
	usersFile    string
	matchesFile  string
}

func NewUserStore() (*UserStore, error) {
	store := &UserStore{
		users:       make(map[string]string),
		lastMatches: make(map[string]int64),
		usersFile:   "data/users.json",
		matchesFile: "data/last_matches.json",
	}

	// Crear directorio data/ si no existe
	if err := os.MkdirAll("data", 0755); err != nil {
		return nil, fmt.Errorf("error creando directorio data: %w", err)
	}

	// Cargar datos existentes
	if err := store.load(); err != nil {
		return nil, fmt.Errorf("error cargando datos: %w", err)
	}

	return store, nil
}

func (s *UserStore) load() error {
	// Cargar usuarios
	if data, err := os.ReadFile(s.usersFile); err == nil {
		if err := json.Unmarshal(data, &s.users); err != nil {
			return fmt.Errorf("error decodificando usuarios: %w", err)
		}
	}

	// Cargar últimas partidas
	if data, err := os.ReadFile(s.matchesFile); err == nil {
		if err := json.Unmarshal(data, &s.lastMatches); err != nil {
			return fmt.Errorf("error decodificando últimas partidas: %w", err)
		}
	}

	return nil
}

func (s *UserStore) save() error {
	// Guardar usuarios
	usersData, err := json.MarshalIndent(s.users, "", "  ")
	if err != nil {
		return fmt.Errorf("error codificando usuarios: %w", err)
	}
	if err := os.WriteFile(s.usersFile, usersData, 0644); err != nil {
		return fmt.Errorf("error guardando usuarios: %w", err)
	}

	// Guardar últimas partidas
	matchesData, err := json.MarshalIndent(s.lastMatches, "", "  ")
	if err != nil {
		return fmt.Errorf("error codificando últimas partidas: %w", err)
	}
	if err := os.WriteFile(s.matchesFile, matchesData, 0644); err != nil {
		return fmt.Errorf("error guardando últimas partidas: %w", err)
	}

	return nil
}

func (s *UserStore) Set(discordID, dotaID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[discordID] = dotaID
	return s.save()
}

func (s *UserStore) Get(discordID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	dotaID, ok := s.users[discordID]
	return dotaID, ok
}

func (s *UserStore) GetAll() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]string)
	for k, v := range s.users {
		result[k] = v
	}
	return result
}

func (s *UserStore) SetLastMatch(discordID string, matchID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastMatches[discordID] = matchID
	return s.save()
}

func (s *UserStore) GetLastMatch(discordID string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	matchID, ok := s.lastMatches[discordID]
	return matchID, ok
}

func (s *UserStore) SetChannel(channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Guardar canal en un archivo separado
	channelData := map[string]string{"channel_id": channelID}
	data, err := json.MarshalIndent(channelData, "", "  ")
	if err != nil {
		return fmt.Errorf("error codificando canal: %w", err)
	}
	if err := os.WriteFile("data/notification_channel.json", data, 0644); err != nil {
		return fmt.Errorf("error guardando canal: %w", err)
	}
	return nil
}

func (s *UserStore) GetChannel() (string, error) {
	data, err := os.ReadFile("data/notification_channel.json")
	if err != nil {
		return "", nil // No es error si no existe
	}
	var channelData map[string]string
	if err := json.Unmarshal(data, &channelData); err != nil {
		return "", fmt.Errorf("error decodificando canal: %w", err)
	}
	return channelData["channel_id"], nil
}

