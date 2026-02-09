package utils

import (
	"crypto/rand"
	"math/big"

	"github.com/google/uuid"
)

func RandomUUID() string {
	return uuid.New().String()
}

func RandomPort() int {
	max := big.NewInt(65535 - 1024 + 1)
	n, _ := rand.Int(rand.Reader, max)
	return int(n.Int64()) + 1024
}
