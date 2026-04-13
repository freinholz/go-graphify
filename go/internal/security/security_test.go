package security

import "testing"

func TestValidateURL(t *testing.T) {
	cases := []struct {
		in  string
		ok  bool
	}{
		{"https://example.com/x", true},
		{"http://example.com", true},
		{"file:///etc/passwd", false},
		{"ftp://host/x", false},
		{"https://127.0.0.1", false},
		{"https://169.254.169.254", false},
	}
	for _, c := range cases {
		_, err := ValidateURL(c.in)
		if (err == nil) != c.ok {
			t.Errorf("%q: ok=%v err=%v", c.in, c.ok, err)
		}
	}
}

func TestSanitizeLabel(t *testing.T) {
	got := SanitizeLabel("hello\x00world<script>")
	if got == "" || len(got) > MaxLabelLen {
		t.Fatalf("unexpected: %q", got)
	}
	if got == "hello\x00world<script>" {
		t.Fatalf("not sanitized: %q", got)
	}
}
