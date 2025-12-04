package logging

import "time"

type AccessEntry struct {
	Timestamp time.Time `json:"ts"`
	Action    string    `json:"action"`
	Depth     string    `json:"depth"`
}

type WriteEntry struct {
	Timestamp time.Time `json:"ts"`
	Operation string    `json:"op"`
	Action    string    `json:"action"`
	Parent    string    `json:"parent,omitempty"`
}