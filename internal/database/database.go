package database

// Service defines the database lifecycle contract used by the server layer.
type Service interface {
	Health() map[string]string
	Close() error
}
