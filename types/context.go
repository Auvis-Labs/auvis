package types

type RequestContext struct {
	RequestID      string
	UserID         string
	UserRole       string
	Resource       string
	Method         string
	Service        string
	Fields         []string
	UserAttributes map[string]string
	Source         string
	Destination    string
	FieldTags      map[string][]string
	Trigger        string
}