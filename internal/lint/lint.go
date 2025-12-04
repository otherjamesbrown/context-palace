package lint

type LintResult struct {
	Valid    bool          `json:"valid"`
	Errors   []LintError   `json:"errors"`
	Warnings []LintWarning `json:"warnings"`
}

type LintError struct {
	Action   string `json:"action"`
	Field    string `json:"field,omitempty"`
	Error    string `json:"error"`
	Severity string `json:"severity"`
}

type LintWarning struct {
	Action  string `json:"action"`
	Warning string `json:"warning"`
}