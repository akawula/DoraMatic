package codeowners

import (
	"sort"
	"testing"
)

func TestParseCodeowners(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "individual users only - no teams",
			content: `# Code Owners
* @dbrian @thed00de @konstantinosbotonakis @ToughCrab24 @vistiria

# Team members:
# - Brian Gosnell (@dbrian)
`,
			expected: []string{},
		},
		{
			name: "single team owner",
			content: `* @wpengine/plutus

#jira:SWPS is where issues related to this repository should be ticketed
`,
			expected: []string{"wpengine/plutus"},
		},
		{
			name: "codeowners file owner",
			content: `.github/CODEOWNERS @wpengine/golden

#jira:AU
`,
			expected: []string{"wpengine/golden"},
		},
		{
			name: "multiple teams with different paths",
			content: `# Gemfile owners
Gemfile @wpengine/portal-ruby-dependency
Gemfile.lock @wpengine/portal-ruby-dependency

# Unicorn owners
unicorn/ @wpengine/unicorn-reviewers
.storybook/ @wpengine/unicorn-reviewers

# Gaia
playwright @wpengine/gaia
app/views/headless_apps/ @wpengine/gaia

# Identity
app/controllers/admin/single_sign_ons_controller.rb @wpengine/bouncer
app/controllers/api/scim/ @wpengine/bouncer

# remove ownership on all docs/ folders
docs/
catalog-info.yaml
mkdocs.yaml
README.md
`,
			expected: []string{
				"wpengine/portal-ruby-dependency",
				"wpengine/unicorn-reviewers",
				"wpengine/gaia",
				"wpengine/bouncer",
			},
		},
		{
			name: "multiple teams on same line",
			content: `app/webpack/components/AllSites/components/Banners/SuspendedAccounts.tsx @wpengine/athena @wpengine/payup
app/webpack/components/Navigation/TopNav/components/Cart @wpengine/cafe @wpengine/payup
`,
			expected: []string{
				"wpengine/athena",
				"wpengine/payup",
				"wpengine/cafe",
			},
		},
		{
			name: "global team owner",
			content: `* @wpengine/ges-reviewers
`,
			expected: []string{"wpengine/ges-reviewers"},
		},
		{
			name: "empty file",
			content: ``,
			expected: []string{},
		},
		{
			name: "only comments",
			content: `# This is a comment
# Another comment
`,
			expected: []string{},
		},
		{
			name: "lines with no owners",
			content: `docs/
README.md
*.spec.ts
`,
			expected: []string{},
		},
		{
			name: "mixed case team names - should be lowercased",
			content: `* @wpengine/CSI
/src/ @wpengine/Golden-Frontend
`,
			expected: []string{"wpengine/csi", "wpengine/golden-frontend"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCodeowners(tt.content)

			// Sort both slices for comparison
			sort.Strings(result)
			sort.Strings(tt.expected)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d teams, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			for i, team := range result {
				if team != tt.expected[i] {
					t.Errorf("expected team %s at index %d, got %s", tt.expected[i], i, team)
				}
			}
		})
	}
}
