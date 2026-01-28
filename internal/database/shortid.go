package database

import (
	"crypto/rand"
	"math/big"
)

const (
	shortIDLength  = 15
	shortIDCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

func GenerateShortID() string {
	result := make([]byte, shortIDLength)
	charsetLen := big.NewInt(int64(len(shortIDCharset)))

	for i := 0; i < shortIDLength; i++ {
		num, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			num = big.NewInt(0)
		}
		result[i] = shortIDCharset[num.Int64()]
	}

	return string(result)
}
