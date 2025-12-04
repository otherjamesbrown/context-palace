package action

import "time"

type Action struct {
	Name      string                 `yaml:"-" json:"name"`
	Path      string                 `yaml:"-" json:"path"`
	Parent    string                 `yaml:"parent,omitempty" json:"parent"`
	SourceDoc string                 `yaml:"source_doc,omitempty" json:"source_doc"`
	Children  []string               `yaml:"-" json:"children"`
	Content   map[string]interface{} `yaml:",inline" json:"content"`
}

type ActionMeta struct {
	Name     string   `json:"name"`
	Parent   *string  `json:"parent"`
	Children []string `json:"children"`
}