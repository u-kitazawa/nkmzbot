package voice

import (
	"sync"
)

type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

func (m *Manager) GetSession(guildID string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[guildID]
}

func (m *Manager) AddSession(guildID string, session *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[guildID] = session
}

func (m *Manager) RemoveSession(guildID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, guildID)
}
