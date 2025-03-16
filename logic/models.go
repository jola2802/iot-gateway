package logic

// <---------------------------------------->
// Models for user_manager.go
// <---------------------------------------->

// Auth kapselt Informationen zur Authentifizierung
type Auth struct {
	Username string
	Password string
	Allow    bool
}

// Filters stellt eine Menge von Filterwerten für Benutzer dar
type Filters map[string]int

// ACL kapselt eine Access Control List
type ACL struct {
	Username string
	Filters  Filters
}

// Command Definition
type Command struct {
	Action   string         `json:"action"`
	Username string         `json:"username"`
	Password string         `json:"password"`
	Allow    bool           `json:"allow"`
	Filters  map[string]int `json:"filters,omitempty"`
}
