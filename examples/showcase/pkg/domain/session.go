package domain

// Session is the authenticated demo identity attached to a request.
type Session struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}
