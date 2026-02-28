package account

import "testing"

func TestGenerateUUID(t *testing.T) {
	id := GenerateUUID()
	if id == "" {
		t.Fatal("GenerateUUID() returned empty string")
	}
	if !IsValidUUID(id) {
		t.Fatalf("GenerateUUID() returned invalid uuid: %q", id)
	}
}

func TestIsValidUUID(t *testing.T) {
	if !IsValidUUID("550e8400-e29b-41d4-a716-446655440000") {
		t.Fatal("IsValidUUID(valid) = false, want true")
	}
	if IsValidUUID("invalid-uuid") {
		t.Fatal("IsValidUUID(invalid) = true, want false")
	}
}
