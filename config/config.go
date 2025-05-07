package config

import (
	"encoding/json"
	"os"
	"sync"
)

// Filters defines the structure for email filtering rules.
type Filters struct {
	IgnoreSenders           []string `json:"ignoreSenders"`
	IgnoreKeywordsInSubject []string `json:"ignoreKeywordsInSubject"`
	IgnoreKeywordsInBody    []string `json:"ignoreKeywordsInBody"` // TODO: Implement body keyword filtering
}

// Manager handles loading, saving, and accessing filter configurations.
type Manager struct {
	filePath string
	filters  *Filters
	mu       sync.RWMutex
}

// NewManager creates a new filter manager.
func NewManager(filePath string) (*Manager, error) {
	m := &Manager{
		filePath: filePath,
		filters:  &Filters{}, // Initialize with empty filters
	}
	err := m.LoadFilters()
	if err != nil {
		// If file doesn't exist, it's fine, we'll use empty filters and save later
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return m, nil
}

// LoadFilters loads filter rules from the JSON file.
func (m *Manager) LoadFilters() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		// If file doesn't exist, initialize with empty filters
		if os.IsNotExist(err) {
			m.filters = &Filters{
				IgnoreSenders:           []string{},
				IgnoreKeywordsInSubject: []string{},
				IgnoreKeywordsInBody:    []string{},
			}
			return m.saveFilters() // Create the file with empty structure
		}
		return err
	}

	var filters Filters
	if err := json.Unmarshal(data, &filters); err != nil {
		return err
	}
	m.filters = &filters
	return nil
}

// saveFilters saves the current filter rules to the JSON file.
// This is an internal method, public methods should be used for modifications.
func (m *Manager) saveFilters() error {
	data, err := json.MarshalIndent(m.filters, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath, data, 0644)
}

// GetFilters returns a copy of the current filters.
func (m *Manager) GetFilters() Filters {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Return a copy to prevent external modification of the internal state
	f := *m.filters
	return f
}

// AddIgnoreSender adds a sender to the ignore list and saves.
func (m *Manager) AddIgnoreSender(sender string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Avoid duplicates
	for _, s := range m.filters.IgnoreSenders {
		if s == sender {
			return nil // Already exists
		}
	}
	m.filters.IgnoreSenders = append(m.filters.IgnoreSenders, sender)
	return m.saveFilters()
}

// AddIgnoreKeywordInSubject adds a subject keyword to the ignore list and saves.
func (m *Manager) AddIgnoreKeywordInSubject(keyword string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range m.filters.IgnoreKeywordsInSubject {
		if k == keyword {
			return nil
		}
	}
	m.filters.IgnoreKeywordsInSubject = append(m.filters.IgnoreKeywordsInSubject, keyword)
	return m.saveFilters()
}

// TODO: Add functions to remove filters
// TODO: Add functions for body keywords
