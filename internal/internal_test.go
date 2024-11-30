package internal_test

import (
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"noobular/internal"
	"noobular/internal/db"
	"os"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/assert"
)

const testUrl = "http://localhost:8080"
const testJwtSecretHex = "5b0c060a53f2c6cd88dde0993fac31648ae75fe092b56571e6b51da56a8e4e87"

func testServer(t *testing.T, ready chan struct{}) {
	jwtSecret, err := hex.DecodeString(testJwtSecretHex)
	assert.Nil(t, err)
	urlStr := testUrl
	urlUrl, err := url.Parse(urlStr)
	assert.Nil(t, err)
	webAuthn, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "WebAuthn Demo",   // Display Name for your site
		RPID:          urlUrl.Hostname(), // Generally the domain name for your site
		RPOrigins:     []string{urlStr},  // The origin URL for WebAuthn requests
	})
	assert.Nil(t, err)

	port := 8080
	dbClient := db.NewMemoryDbClient()
	defer dbClient.Close()
	os.Chdir("..") // Go to root of this project
	server := internal.NewServer(dbClient, webAuthn, jwtSecret, port)
	close(ready)
	err = server.ListenAndServe()
	assert.Nil(t, err)
}

func startServer(t *testing.T) {
	ready := make(chan struct{})
	go testServer(t, ready)
	<-ready
}

func TestBasicNav(t *testing.T) {
	startServer(t)

	tests := []struct {
		name         string
		path         string
		expectedText string
	}{
		{"home", "/", "Welcome to Noobular"},
		{"browse", "/browse", "Courses"},
	}

	test := func(t *testing.T, path string, expectedText string) {
		resp, err := http.Get(testUrl + path)
		assert.Nil(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		bodyBytes, err := io.ReadAll(resp.Body)
		assert.Nil(t, err)
		body := string(bodyBytes)
		assert.Contains(t, body, expectedText)
		assert.Contains(t, body, "Signin")
		assert.Contains(t, body, "Signup")

		req, err := http.NewRequest("GET", testUrl + path, nil)
		assert.Nil(t, err)

		jwtSecret, err := hex.DecodeString(testJwtSecretHex)
		assert.Nil(t, err)
		userId := int64(1)
		cookie, err := internal.CreateAuthCookie(jwtSecret, userId)
		assert.Nil(t, err)
		req.AddCookie(&cookie)
		resp, err = http.DefaultClient.Do(req)
		assert.Nil(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		bodyBytes, err = io.ReadAll(resp.Body)
		assert.Nil(t, err)
		body = string(bodyBytes)
		assert.Contains(t, body, expectedText)
		assert.Contains(t, body, "Logout")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			test(t, tt.path, tt.expectedText)
		})
	}

}
