package logging

import "time"

type AccessEntry struct {
	Timestamp time.Time `json:"ts"`
	Memo      string    `json:"memo"`
	Depth     string    `json:"depth"`
}

type WriteEntry struct {
	Timestamp time.Time `json:"ts"`
	Operation string    `json:"op"`
	Memo      string    `json:"memo"`
	Parent    string    `json:"parent,omitempty"`
}
