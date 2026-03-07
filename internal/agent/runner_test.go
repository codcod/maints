package agent

import (
	"testing"
)

func TestCoalesce(t *testing.T) {
	tests := []struct {
		name string
		vals []string
		want string
	}{
		{"all empty", []string{"", "", ""}, ""},
		{"first non-empty", []string{"a", "b", "c"}, "a"},
		{"skip leading empty", []string{"", "b", "c"}, "b"},
		{"single value", []string{"x"}, "x"},
		{"no values", []string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := coalesce(tt.vals...)
			if got != tt.want {
				t.Errorf("coalesce(%v) = %q, want %q", tt.vals, got, tt.want)
			}
		})
	}
}

func TestParseOutput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:  "single JSON with text field",
			input: `{"text":"hello world"}`,
			want:  "hello world",
		},
		{
			name:  "single JSON with message field",
			input: `{"message":"from message"}`,
			want:  "from message",
		},
		{
			name:  "single JSON with result field",
			input: `{"result":"from result"}`,
			want:  "from result",
		},
		{
			name:  "text field takes priority over message and result",
			input: `{"text":"primary","message":"secondary","result":"tertiary"}`,
			want:  "primary",
		},
		{
			name:  "message field takes priority over result",
			input: `{"message":"secondary","result":"tertiary"}`,
			want:  "secondary",
		},
		{
			name:  "NDJSON stream returns last non-empty text",
			input: "{\"text\":\"first line\"}\n{\"text\":\"second line\"}",
			want:  "second line",
		},
		{
			name:  "NDJSON with blank intermediate lines",
			input: "{\"text\":\"a\"}\n\n{\"text\":\"b\"}",
			want:  "b",
		},
		{
			name:  "raw fallback for non-JSON",
			input: "plain text output",
			want:  "plain text output",
		},
		{
			name:  "JSON object with no recognized text fields returns raw",
			input: `{"unknown":"value"}`,
			want:  `{"unknown":"value"}`,
		},
		{
			name:    "whitespace-only input treated as empty",
			input:   "   \n\t  ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOutput([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("parseOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}
