package internal

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func CreateJwt(jwtSecret []byte, userId int64, expiry time.Time) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId":         userId,
		"expirationDate": expiry.Unix(),
	})
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func ValidateJwt(jwtSecret []byte, tokenString string) (int64, error) {
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	}
	token, err := jwt.Parse(tokenString, keyFunc)
	if err != nil {
		return 0, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, fmt.Errorf("Invalid claims")
	}
	tokenExpiry := int64(claims["expirationDate"].(float64))
	if time.Now().Unix() > tokenExpiry {
		return 0, fmt.Errorf("Token has expired, expiry: %d", tokenExpiry)
	}
	return int64(claims["userId"].(float64)), nil
}
