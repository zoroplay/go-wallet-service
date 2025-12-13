package common

import (
	"math/rand"
	"time"
)

func GenerateTrxNo() string {
	const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	result := make([]byte, 7)
	for i := range result {
		result[i] = characters[r.Intn(len(characters))]
	}
	return string(result)
}
