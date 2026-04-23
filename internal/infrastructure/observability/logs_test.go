package observability

import "testing"

func TestNormalizeOTLPEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     string
		wantErr  bool
	}{
		{name: "host and port", input: "signoz-otel-collector:4318", want: "signoz-otel-collector:4318"},
		{name: "http url", input: "http://signoz-otel-collector:4318", want: "signoz-otel-collector:4318"},
		{name: "https url", input: "https://signoz-otel-collector:4318", want: "signoz-otel-collector:4318"},
		{name: "invalid url", input: "http://", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeOTLPEndpoint(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
