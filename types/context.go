package types

type RequestContext struct {
	RequestID string
	UserID    string
	UserRole  string
	Resource  string
	Method    string
}