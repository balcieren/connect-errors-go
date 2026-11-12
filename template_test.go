package connecterrors_test

import (
	"testing"

	connecterrors "github.com/balcieren/connect-errors-go/v2"
)

func TestTemplateFields(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     []string
	}{
		{"single field", "User '{{id}}' not found", []string{"id"}},
		{"multiple fields", "User '{{id}}' in org '{{org}}'", []string{"id", "org"}},
		{"duplicate fields", "{{id}} and {{id}} again", []string{"id"}},
		{"no fields", "Static error message", nil},
		{"empty string", "", nil},
		{"three fields sorted", "{{zebra}} {{apple}} {{mango}}", []string{"apple", "mango", "zebra"}},
		{"underscore field", "user_id={{user_id}}", []string{"user_id"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := connecterrors.TemplateFields(tt.template)
			if len(got) != len(tt.want) {
				t.Fatalf("TemplateFields(%q) = %v, want %v", tt.template, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("field[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFormatTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     connecterrors.M
		want     string
	}{
		{"single replacement", "User '{{id}}' not found", connecterrors.M{"id": "123"}, "User '123' not found"},
		{"multiple replacements", "{{actor}} cannot update {{target}}", connecterrors.M{"actor": "alice", "target": "bob"}, "alice cannot update bob"},
		{"missing field unchanged", "User '{{id}}' in {{org}}", connecterrors.M{"id": "123"}, "User '123' in {{org}}"},
		{"nil data", "User '{{id}}'", nil, "User '{{id}}'"},
		{"empty data", "User '{{id}}'", connecterrors.M{}, "User '{{id}}'"},
		{"no placeholders", "Internal error", connecterrors.M{"id": "123"}, "Internal error"},
		{"special chars in value", "Email '{{email}}'", connecterrors.M{"email": "a@b.com"}, "Email 'a@b.com'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := connecterrors.FormatTemplate(tt.template, tt.data)
			if got != tt.want {
				t.Errorf("FormatTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateTemplate(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		data      connecterrors.M
		wantErr   bool
		wantCount int
	}{
		{"all fields provided", "User '{{id}}'", connecterrors.M{"id": "123"}, false, 0},
		{"missing field", "User '{{id}}'", connecterrors.M{}, true, 1},
		{"multiple missing", "{{actor}} {{target}}", connecterrors.M{}, true, 2},
		{"no placeholders", "Static message", connecterrors.M{}, false, 0},
		{"extra fields ok", "User '{{id}}'", connecterrors.M{"id": "123", "extra": "v"}, false, 0},
		{"partial missing", "{{id}} in {{org}}", connecterrors.M{"id": "123"}, true, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := connecterrors.ValidateTemplate(tt.template, tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateTemplate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				mfe, ok := err.(*connecterrors.MissingFieldError)
				if !ok {
					t.Fatalf("expected *MissingFieldError, got %T", err)
				}
				if len(mfe.Missing) != tt.wantCount {
					t.Errorf("missing count = %d, want %d", len(mfe.Missing), tt.wantCount)
				}
			}
		})
	}
}

func TestMissingFieldErrorMessage(t *testing.T) {
	err := &connecterrors.MissingFieldError{
		Template: "User '{{id}}'",
		Missing:  []string{"id"},
	}
	want := "template \"User '{{id}}'\" missing fields: id"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
