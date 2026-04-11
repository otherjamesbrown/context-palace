package client

import "testing"

func TestParseGitHubOwnerRepo(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{
			name: "ssh with .git suffix",
			url:  "git@github.com:otherjamesbrown/context-palace.git",
			want: "otherjamesbrown/context-palace",
		},
		{
			name: "ssh without .git suffix",
			url:  "git@github.com:otherjamesbrown/context-palace",
			want: "otherjamesbrown/context-palace",
		},
		{
			name: "https with .git suffix",
			url:  "https://github.com/otherjamesbrown/context-palace.git",
			want: "otherjamesbrown/context-palace",
		},
		{
			name: "https without .git suffix",
			url:  "https://github.com/otherjamesbrown/context-palace",
			want: "otherjamesbrown/context-palace",
		},
		{
			name: "ssh protocol scheme",
			url:  "ssh://git@github.com/otherjamesbrown/context-palace.git",
			want: "otherjamesbrown/context-palace",
		},
		{
			name: "repo name with hyphens and numbers",
			url:  "git@github.com:my-org/my-repo-2.git",
			want: "my-org/my-repo-2",
		},
		{
			name: "https with trailing slash",
			url:  "https://github.com/owner/repo/",
			want: "owner/repo",
		},
		{
			name:    "not a github remote",
			url:     "git@gitlab.com:owner/repo.git",
			wantErr: true,
		},
		{
			name:    "empty string",
			url:     "",
			wantErr: true,
		},
		{
			name:    "github.com with no path",
			url:     "https://github.com",
			wantErr: true,
		},
		{
			name:    "github.com with only owner",
			url:     "https://github.com/owner",
			wantErr: true,
		},
		{
			name:    "github.com with missing owner",
			url:     "https://github.com//repo",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGitHubOwnerRepo(tc.url)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %q", tc.url, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.url, err)
			}
			if got != tc.want {
				t.Errorf("parseGitHubOwnerRepo(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}
