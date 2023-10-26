package utils

import (
	"crypto/rand"
	"math/big"
)

const (
	chars = "0123456789abcdefghijklmnopqrstuvwxyz"
)

func GenerateRandomString(length int) (string, error) {
	var result string
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		// I don't think the above can ever error? unless ken thompson is stinky
		if err != nil {
			return "", err
		}
		result += string(chars[num.Int64()])
	}
	return result, nil
}
