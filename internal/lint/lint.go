package lint

type LintResult struct {
	Valid    bool          `json:"valid"`
	Errors   []LintError   `json:"errors"`
	Warnings []LintWarning `json:"warnings"`
}

type LintError struct {
	Memo     string `json:"memo"`
	Field    string `json:"field,omitempty"`
	Error    string `json:"error"`
	Severity string `json:"severity"`
}

type LintWarning struct {
	Memo    string `json:"memo"`
	Warning string `json:"warning"`
}
