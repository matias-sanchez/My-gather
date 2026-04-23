package parse

import "testing"

func TestParseEnvHostname_LastNonWarningLine(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "eu-hrznp-d003.iconcr.com\n", "eu-hrznp-d003.iconcr.com"},
		{"trailing blank", "eu-hrznp-d003\n\n", "eu-hrznp-d003"},
		{"sudo warning skipped", "sudo: unable to resolve host foo\nhost-01\n", "host-01"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ParseEnvHostname(c.input); got != c.want {
				t.Errorf("ParseEnvHostname(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}
