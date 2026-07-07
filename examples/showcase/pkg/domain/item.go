package domain

// Item is an in-memory demo catalog entity.
type Item struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
