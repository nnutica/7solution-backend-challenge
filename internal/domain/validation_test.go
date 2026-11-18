package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"valid", "Alice", "Alice", false},
		{"trims spaces", "  Alice  ", "Alice", false},
		{"empty", "", "", true},
		{"only spaces", "   ", "", true},
		{"max length", strings.Repeat("a", MaxNameLength), strings.Repeat("a", MaxNameLength), false},
		{"too long", strings.Repeat("a", MaxNameLength+1), "", true},
		{"multibyte counts runes", strings.Repeat("ก", MaxNameLength), strings.Repeat("ก", MaxNameLength), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeName(tt.input)
			assertValidation(t, err, tt.wantErr, "name")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"valid", "alice@example.com", "alice@example.com", false},
		{"lowercases and trims", "  Alice@Example.COM ", "alice@example.com", false},
		{"empty", "", "", true},
		{"missing at", "alice.example.com", "", true},
		{"missing local part", "@example.com", "", true},
		{"display name form", "Alice <alice@example.com>", "", true},
		{"two addresses", "a@x.com, b@x.com", "", true},
		{"too long", strings.Repeat("a", MaxEmailLength) + "@x.com", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeEmail(tt.input)
			assertValidation(t, err, tt.wantErr, "email")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"min length", strings.Repeat("a", MinPasswordLength), false},
		{"too short", strings.Repeat("a", MinPasswordLength-1), true},
		{"max length", strings.Repeat("a", MaxPasswordLength), false},
		{"too long", strings.Repeat("a", MaxPasswordLength+1), true},
		{"spaces kept", "pass word", false},
		{"multibyte over byte limit", strings.Repeat("ก", 25), true}, // 75 bytes
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidation(t, ValidatePassword(tt.input), tt.wantErr, "password")
		})
	}
}

func assertValidation(t *testing.T, err error, wantErr bool, field string) {
	t.Helper()
	if !wantErr {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want *ValidationError, got %v", err)
	}
	if ve.Field != field {
		t.Errorf("field = %q, want %q", ve.Field, field)
	}
}
