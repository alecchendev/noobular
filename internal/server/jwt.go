package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func CreateAuthCookie(jwtSecret []byte, userId int64, httpsOnly bool) (http.Cookie, error) {
	expiry := time.Now().Add(2 * 7 * 24 * time.Hour) // 2 weeks
	token, err := CreateJwt(jwtSecret, userId, expiry)
	if err != nil {
		return http.Cookie{}, err
	}
	return http.Cookie{
		Name:     "session_token",
		Value:    token,
		Expires:  expiry,
		HttpOnly: true,                 // Not accessible to client side code
		SameSite: http.SameSiteLaxMode, // Cannot send cookie to other domains
		Secure:   httpsOnly,            // HTTPS only, need to disable locally
		Path:     "/",
	}, nil
}

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
