package internal_test

import (
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"noobular/internal"
	"noobular/internal/db"
	"strconv"
	"strings"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/assert"
)

const testUrl = "http://localhost:8080"
const testJwtSecretHex = "5b0c060a53f2c6cd88dde0993fac31648ae75fe092b56571e6b51da56a8e4e87"

func testServer() *http.Server {
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
	return internal.NewServer(dbClient, renderer, webAuthn, jwtSecret, port)
}

func startServer() *http.Server {
	server := testServer()
	ready := make(chan struct{})
	go func() {
		close(ready)
		server.ListenAndServe()
	}()
	<-ready
	return server
}

type testClient struct {
	t             *testing.T
	baseUrl       string
	session_token *http.Cookie
}

func newTestClient(t *testing.T) testClient {
	return testClient{t: t, baseUrl: testUrl}
}

func (c testClient) request(method string, path string, body string) *http.Response {
	req, _ := http.NewRequest(method, c.baseUrl+path, strings.NewReader(body))
	if method == "POST" || method == "PUT" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if c.session_token != nil {
		req.AddCookie(c.session_token)
	}
	resp, _ := http.DefaultClient.Do(req)
	return resp
}

func (c testClient) get(path string) *http.Response {
	return c.request("GET", path, "")
}

func (c testClient) post(path string, body string) *http.Response {
	return c.request("POST", path, body)
}

func (c testClient) put(path string, body string) *http.Response {
	return c.request("PUT", path, body)
}

func (c testClient) login(userId int64) testClient {
	jwtSecret, _ := hex.DecodeString(testJwtSecretHex)
	cookie, _ := internal.CreateAuthCookie(jwtSecret, userId)
	c.session_token = &cookie
	return c
}

func (c testClient) createTestUser() testClient {
	resp := c.post("/signup/test", "")
	assert.Equal(c.t, 200, resp.StatusCode)
	return c.login(1)
}

const createCourseRoute = "/teacher/course/create"

func (c testClient) createCourse(course db.Course, modules []db.Module) {
	formData := createCourseForm(course, modules)
	resp := c.post(createCourseRoute, formData.Encode())
	assert.Equal(c.t, 200, resp.StatusCode)
}

func createCourseForm(course db.Course, modules []db.Module) url.Values {
	formData := url.Values{}
	formData.Set("title", course.Title)
	formData.Set("description", course.Description)
	for _, module := range modules {
		formData.Add("module-title[]", module.Title)
		formData.Add("module-id[]", "-1")
		formData.Add("module-description[]", module.Description)
	}
	return formData
}
type blockInput struct {
	blockType db.BlockType
	block interface{}
}

func editModulePageRoute(courseId, moduleId int) string {
	return editModuleRoute(courseId, moduleId) + "/edit"
}

func editModuleRoute(courseId, moduleId int) string {
	return fmt.Sprintf("/teacher/course/%d/module/%d", courseId, moduleId)
}

func (c testClient) editModule(module db.Module, blocks []blockInput) {
	formData := editModuleForm(module, blocks)
	resp := c.put(editModuleRoute(module.CourseId, module.Id), formData.Encode())
	assert.Equal(c.t, 200, resp.StatusCode)
}

func editModuleForm(module db.Module, blocks []blockInput) url.Values {
	formData := url.Values{}
	formData.Set("title", module.Title)
	formData.Set("description", module.Description)
	for _, block := range blocks {
		formData.Add("block-type[]", string(block.blockType))
		switch block.blockType {
		case db.QuestionBlockType:
			question := block.block.(internal.UiQuestion)
			formData.Add("question-title[]", question.QuestionText)
			formData.Add("question-idx[]", strconv.Itoa(question.Idx))
			formData.Add("question-explanation[]", question.Explanation)
			for _, choice := range question.Choices {
				formData.Add("choice-title[]", choice.ChoiceText)
				formData.Add("choice-idx[]", strconv.Itoa(choice.Idx))
				if choice.IsCorrect {
					formData.Add("correct-choice-"+strconv.Itoa(choice.QuestionIdx), strconv.FormatBool(true))
				}
			}
			formData.Add("choice-title[]", "end-choice")
			formData.Add("choice-idx[]", "end-choice")
		case db.ContentBlockType:
			formData.Add("content-text[]", block.block.(db.Content).Content)
		}
	}
	return formData
}

func bodyText(t *testing.T, resp *http.Response) string {
	bodyBytes, err := io.ReadAll(resp.Body)
	assert.Nil(t, err)
	return string(bodyBytes)
}

func TestBasicNav(t *testing.T) {
	server := startServer()
	defer server.Close()

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
	server := startServer()
	defer server.Close()

	client := newTestClient(t).createTestUser()

	resp := client.get("/teacher")
	assert.Equal(t, 200, resp.StatusCode)

	course := db.NewCourse(-1, "hello", "goodbye")
	modules := []db.Module{
		db.NewModule(-1, -1, "module title1", "module description1"),
		db.NewModule(-1, -1, "module title2", "module description2"),
	}

	body := bodyText(t, resp)
	assert.Contains(t, body, createCourseRoute)
	assert.NotContains(t, body, course.Title)

	client.createCourse(course, modules)

	resp = client.get("/teacher")
	assert.Equal(t, 200, resp.StatusCode)
	body = bodyText(t, resp)
	assert.Contains(t, body, course.Title)
	assert.Contains(t, body, course.Description)
	for _, module := range modules {
		assert.Contains(t, body, module.Title)
		assert.Contains(t, body, module.Description)
	}
}

func TestEditModule(t *testing.T) {
	server := startServer()
	defer server.Close()

	client := newTestClient(t).createTestUser()

	course := db.NewCourse(-1, "hello", "goodbye")
	modules := []db.Module{
		db.NewModule(-1, -1, "module title1", "module description1"),
	}
	client.createCourse(course, modules)

	module := db.NewModule(1, 1, "new title", "new description")

	resp := client.get("/teacher")
	assert.Equal(t, 200, resp.StatusCode)
	body := bodyText(t, resp)
	editModulePageLink := editModulePageRoute(module.CourseId, module.Id)
	assert.Contains(t, body, editModulePageLink)

	resp = client.get(editModulePageLink)
	assert.Equal(t, 200, resp.StatusCode)
	body = bodyText(t, resp)
	assert.Contains(t, body, editModuleRoute(module.CourseId, module.Id))

	blocks := []blockInput{
		{db.QuestionBlockType, internal.NewUiQuestion(db.NewQuestion(-1, -1, "qname1"), []db.Choice{
			db.NewChoice(-1, -1, "qchoice1", false),
			db.NewChoice(-1, -1, "qchoice2", true),
			db.NewChoice(-1, -1, "qchoice3", false),
		}, db.NewContent(-1, "qexplanation1"))},
		{db.ContentBlockType, db.NewContent(-1, "qcontent1")},
		{db.QuestionBlockType, internal.NewUiQuestion(db.NewQuestion(-1, -1, "qname2"), []db.Choice{
			db.NewChoice(-1, -1, "qchoice4", false),
			db.NewChoice(-1, -1, "qchoice5", false),
			db.NewChoice(-1, -1, "qchoice6", true),
		}, db.NewContent(-1, "qexplanation2"))},
		{db.ContentBlockType, db.NewContent(-1, "qcontent1")},
	}

	client.editModule(module, blocks)

	// Check that if we revisit the edit module page
	// all of our changes are reflected
	resp = client.get(editModulePageLink)
	assert.Equal(t, 200, resp.StatusCode)
	body = bodyText(t, resp)
	assert.Contains(t, body, module.Title)
	assert.Contains(t, body, module.Description)
	for _, block := range blocks {
		switch block.blockType {
		case db.QuestionBlockType:
			question := block.block.(internal.UiQuestion)
			assert.Contains(t, body, question.QuestionText)
			assert.Contains(t, body, question.Explanation)
			for _, choice := range question.Choices {
				assert.Contains(t, body, choice.ChoiceText)
			}
		case db.ContentBlockType:
			content := block.block.(db.Content)
			assert.Contains(t, body, content.Content)
		}
	}
}
