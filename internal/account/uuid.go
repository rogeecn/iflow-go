package account

import "github.com/google/uuid"

func GenerateUUID() string {
	return uuid.NewString()
}

func IsValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}
