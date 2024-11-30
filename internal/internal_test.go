package internal_test

import (
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"noobular/internal"
	"noobular/internal/db"
	"strings"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/assert"
)

const testUrl = "http://localhost:8080"
const testJwtSecretHex = "5b0c060a53f2c6cd88dde0993fac31648ae75fe092b56571e6b51da56a8e4e87"

func testServer(ready chan struct{}) {
	jwtSecret, _ := hex.DecodeString(testJwtSecretHex)
	urlStr := testUrl
	urlUrl, _ := url.Parse(urlStr)
	webAuthn, _ := webauthn.New(&webauthn.Config{
		RPDisplayName: "WebAuthn Demo",   // Display Name for your site
		RPID:          urlUrl.Hostname(), // Generally the domain name for your site
		RPOrigins:     []string{urlStr},  // The origin URL for WebAuthn requests
	})

	port := 8080
	renderer := internal.NewRenderer("..")
	dbClient := db.NewMemoryDbClient()
	server := internal.NewServer(dbClient, renderer, webAuthn, jwtSecret, port)
	close(ready)
	server.ListenAndServe()
}

func startServer() {
	ready := make(chan struct{})
	go testServer(ready)
	<-ready
}

type testClient struct {
	t             *testing.T
	baseUrl       string
	session_token *http.Cookie
}

func newTestClient(t *testing.T) testClient {
	return testClient{t: t, baseUrl: testUrl}
}

func (c testClient) login(userId int64) testClient {
	jwtSecret, _ := hex.DecodeString(testJwtSecretHex)
	cookie, _ := internal.CreateAuthCookie(jwtSecret, userId)
	c.session_token = &cookie
	return c
}

func (c testClient) get(path string) *http.Response {
	req, _ := http.NewRequest("GET", c.baseUrl+path, nil)
	if c.session_token != nil {
		req.AddCookie(c.session_token)
	}
	resp, _ := http.DefaultClient.Do(req)
	return resp
}

func (c testClient) post(path string, body string) *http.Response {
	req, _ := http.NewRequest("POST", c.baseUrl+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c.session_token != nil {
		req.AddCookie(c.session_token)
	}
	resp, _ := http.DefaultClient.Do(req)
	return resp
}

func bodyText(t *testing.T, resp *http.Response) string {
	bodyBytes, err := io.ReadAll(resp.Body)
	assert.Nil(t, err)
	return string(bodyBytes)
}

func TestBasicNav(t *testing.T) {
	startServer()

	tests := []struct {
		name         string
		path         string
		expectedText string
	}{
		{"home", "/", "Welcome to Noobular"},
		{"browse", "/browse", "Courses"},
	}

	test := func(t *testing.T, path string, expectedText string) {
		client := newTestClient(t)
		resp := client.get(path)
		assert.Equal(t, 200, resp.StatusCode)

		body := bodyText(t, resp)
		assert.Contains(t, body, expectedText)
		assert.Contains(t, body, "Signin")
		assert.Contains(t, body, "Signup")

		client = client.login(1)
		resp = client.get(path)
		assert.Equal(t, 200, resp.StatusCode)

		body = bodyText(t, resp)
		assert.Contains(t, body, expectedText)
		assert.Contains(t, body, "Logout")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			test(t, tt.path, tt.expectedText)
		})
	}
}

func TestCreateCourse(t *testing.T) {
	startServer()

	client := newTestClient(t)
	resp := client.post("/signup/test", "")
	assert.Equal(t, 200, resp.StatusCode)

	client = client.login(1)
	resp = client.get("/teacher")
	assert.Equal(t, 200, resp.StatusCode)

	body := bodyText(t, resp)
	assert.Contains(t, body, "/teacher/course/create")
	sampleCourseName := "sample course name"
	assert.NotContains(t, body, sampleCourseName)

	formData := url.Values{}

	formData.Set("title", "hello")
	formData.Set("description", "goodbye")
	formData.Add("module-title[]", "module title")
	formData.Add("module-id[]", "-1")
	formData.Add("module-description[]", "module description")

	resp = client.post("/teacher/course/create", formData.Encode())
	assert.Equal(t, 200, resp.StatusCode)

	resp = client.get("/teacher")
	assert.Equal(t, 200, resp.StatusCode)
	body = bodyText(t, resp)
	assert.Contains(t, body, "hello")
	assert.Contains(t, body, "goodbye")
	assert.Contains(t, body, "module title")
	assert.Contains(t, body, "module description")
}
