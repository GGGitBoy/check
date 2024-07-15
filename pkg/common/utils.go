package common

import "github.com/google/uuid"

func GetUUID() string {
	uuid := uuid.New()
	return uuid.String()
}
