package repos

import "time"

type Repository struct {
	ID        int64     `json:"id"`
	Owner     string    `json:"owner"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateInput struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
}
