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

func testServer(dbClient *db.DbClient) *http.Server {
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
	return internal.NewServer(dbClient, renderer, webAuthn, jwtSecret, port)
}

type testContext struct {
	t *testing.T
	server *http.Server
	db *db.DbClient
}

func (c testContext) Close() {
	c.server.Close()
	c.db.Close()
}

func (c testContext) createUser() db.User {
	user, err := c.db.CreateUser("test")
	assert.Nil(c.t, err)
	return user
}

func startServer() testContext {
	dbClient := db.NewMemoryDbClient()
	server := testServer(dbClient)
	ready := make(chan struct{})
	go func() {
		close(ready)
		server.ListenAndServe()
	}()
	<-ready
	return testContext{server: server, db: dbClient}
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

func (c testClient) getPageBody(path string) string {
	resp := c.get(path)
	assert.Equal(c.t, 200, resp.StatusCode)
	return bodyText(c.t, resp)
}

const createCourseRoute = "/teacher/course/create"

func (c testClient) createCourse(course db.Course, modules []db.ModuleVersion) {
	formData := createOrEditCourseForm(course, modules)
	resp := c.post(createCourseRoute, formData.Encode())
	assert.Equal(c.t, 200, resp.StatusCode)
}

func editCourseRoute(courseId int) string {
	return fmt.Sprintf("/teacher/course/%d", courseId)
}

func editCoursePageRoute(courseId int) string {
	return fmt.Sprintf("/teacher/course/%d", courseId) + "/edit"
}

func (c testClient) editCourse(course db.Course, modules []db.ModuleVersion) {
	formData := createOrEditCourseForm(course, modules)
	resp := c.put(editCourseRoute(course.Id), formData.Encode())
	assert.Equal(c.t, 200, resp.StatusCode)
}

func createOrEditCourseForm(course db.Course, modules []db.ModuleVersion) url.Values {
	formData := url.Values{}
	formData.Set("title", course.Title)
	formData.Set("description", course.Description)
	for _, module := range modules {
		formData.Add("module-title[]", module.Title)
		formData.Add("module-id[]", strconv.Itoa(module.ModuleId))
		formData.Add("module-description[]", module.Description)
	}
	return formData
}
type blockInput struct {
	blockType db.BlockType
	block interface{}
}

func newQuestionBlockInput(question internal.UiQuestion) blockInput {
	return blockInput{db.QuestionBlockType, question}
}

func newContentBlockInput(content string) blockInput {
	return blockInput{db.ContentBlockType, db.NewContent(-1, content)}
}

type uiQuestionBuilder struct {
	t *testing.T
	questionText string
	choices []db.Choice
	explanation string
}

func newUiQuestionBuilder(t *testing.T) uiQuestionBuilder {
	return uiQuestionBuilder{t: t}
}

func (b uiQuestionBuilder) text(text string) uiQuestionBuilder {
	b.questionText = text
	return b
}

func (b uiQuestionBuilder) choice(choiceText string, isCorrect bool) uiQuestionBuilder {
	b.choices = append(b.choices, db.NewChoice(-1, -1, choiceText, isCorrect))
	return b
}

func (b uiQuestionBuilder) explain(text string) uiQuestionBuilder {
	b.explanation = text
	return b
}

func (b uiQuestionBuilder) build() internal.UiQuestion {
	return internal.NewUiQuestionEdit(db.NewQuestion(-1, -1, b.questionText), b.choices, db.NewContent(-1, b.explanation))
}

func editModulePageRoute(courseId, moduleId int) string {
	return editModuleRoute(courseId, moduleId) + "/edit"
}

func editModuleRoute(courseId, moduleId int) string {
	return fmt.Sprintf("/teacher/course/%d/module/%d", courseId, moduleId)
}

func (c testClient) editModule(courseId int, moduleVersion db.ModuleVersion, blocks []blockInput) {
	formData := editModuleForm(moduleVersion, blocks)
	resp := c.put(editModuleRoute(courseId, moduleVersion.ModuleId), formData.Encode())
	assert.Equal(c.t, 200, resp.StatusCode)
}

func editModuleForm(moduleVersion db.ModuleVersion, blocks []blockInput) url.Values {
	formData := url.Values{}
	formData.Set("title", moduleVersion.Title)
	formData.Set("description", moduleVersion.Description)
	for _, block := range blocks {
		formData.Add("block-type[]", string(block.blockType))
		switch block.blockType {
		case db.QuestionBlockType:
			question := block.block.(internal.UiQuestion)
			formData.Add("question-title[]", question.QuestionText)
			formData.Add("question-idx[]", strconv.Itoa(question.Idx))
			formData.Add("question-explanation[]", question.Explanation.Content)
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
	ctx := startServer()
	defer ctx.Close()

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
		body := client.getPageBody(path)
		assert.Contains(t, body, expectedText)
		assert.Contains(t, body, "Signin")
		assert.Contains(t, body, "Signup")

		client = client.login(1)
		body = client.getPageBody(path)
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
	ctx := startServer()
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	course := db.NewCourse(-1, "hello", "goodbye")
	modules := []db.ModuleVersion{
		db.NewModuleVersion(-1, -1, 0, "module title1", "module description1"),
		db.NewModuleVersion(-1, -1, 0, "module title2", "module description2"),
	}

	body := client.getPageBody("/teacher")
	assert.Contains(t, body, createCourseRoute)
	assert.NotContains(t, body, course.Title)

	client.createCourse(course, modules)

	body = client.getPageBody("/teacher")
	assert.Contains(t, body, course.Title)
	assert.Contains(t, body, course.Description)
	for _, module := range modules {
		assert.Contains(t, body, module.Title)
		assert.Contains(t, body, module.Description)
	}
}

func TestEditCourse(t *testing.T) {
	ctx := startServer()
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	course := db.NewCourse(-1, "hello", "goodbye")
	modules := []db.ModuleVersion{
		db.NewModuleVersion(-1, -1, 0, "module title1", "module description1"),
		db.NewModuleVersion(-1, -1, 0, "module title2", "module description2"),
	}

	client.createCourse(course, modules)
	courseId := 1

	body := client.getPageBody("/teacher")
	assert.Contains(t, body, editCourseRoute(courseId))

	newCourse := db.NewCourse(courseId, "new title", "new description")
	newModules := []db.ModuleVersion{
		db.NewModuleVersion(-1, 1, 1, "new module title1", "new module description1"),
		db.NewModuleVersion(-1, 2, 1, "new module title2", "new module description2"),
	}
	client.editCourse(newCourse, newModules)

	for _, route := range []string{"/teacher", editCoursePageRoute(courseId)} {
		body = client.getPageBody(route)
		assert.Contains(t, body, newCourse.Title)
		assert.Contains(t, body, newCourse.Description)
		for _, module := range newModules {
			assert.Contains(t, body, module.Title)
			assert.Contains(t, body, module.Description)
		}
	}
}

func TestEditModule(t *testing.T) {
	ctx := startServer()
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	course := db.NewCourse(-1, "hello", "goodbye")
	modules := []db.ModuleVersion{
		db.NewModuleVersion(-1, -1, 0, "module title1", "module description1"),
	}
	client.createCourse(course, modules)

	courseId := 1
	moduleId := 1

	body := client.getPageBody("/teacher")
	editModulePageLink := editModulePageRoute(courseId, moduleId)
	assert.Contains(t, body, editModulePageLink)

	body = client.getPageBody(editModulePageLink)
	assert.Contains(t, body, editModuleRoute(courseId, moduleId))

	newModuleVersion := db.NewModuleVersion(2, 1, 1, "new title", "new description")

	blocks := []blockInput{
		newQuestionBlockInput(newUiQuestionBuilder(t).
			text("qname1").
			choice("qchoice1", false).
			choice("qchoice2", true).
			choice("qchoice3", false).
			explain("qexplanation1").
			build()),
		newContentBlockInput("qcontent1"),
		newQuestionBlockInput(newUiQuestionBuilder(t).
			text("qname2").
			choice("qchoice4", false).
			choice("qchoice5", false).
			choice("qchoice6", true).
			explain("qexplanation2").
			build()),
		newContentBlockInput("qcontent1"),
	}

	client.editModule(courseId, newModuleVersion, blocks)

	// Check that if we revisit the edit module page
	// all of our changes are reflected
	body = client.getPageBody(editModulePageLink)
	assert.Contains(t, body, newModuleVersion.Title)
	assert.Contains(t, body, newModuleVersion.Description)
	for _, block := range blocks {
		switch block.blockType {
		case db.QuestionBlockType:
			question := block.block.(internal.UiQuestion)
			assert.Contains(t, body, question.QuestionText)
			assert.Contains(t, body, question.Explanation.Content)
			for _, choice := range question.Choices {
				assert.Contains(t, body, choice.ChoiceText)
			}
		case db.ContentBlockType:
			content := block.block.(db.Content)
			assert.Contains(t, body, content.Content)
		}
	}
}

func TestNoDuplicateContent(t *testing.T) {
	ctx := startServer()
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	course := db.NewCourse(-1, "hello", "goodbye")
	modules := []db.ModuleVersion{
		db.NewModuleVersion(-1, -1, 0, "module title1", "module description1"),
	}
	client.createCourse(course, modules)

	courseId := 1
	moduleId := 1

	newModuleVersion := db.NewModuleVersion(2, moduleId, 1, "new title", "new description")
	explanation := "qexplanation1"
	contentStr := "qcontent1"
	question1 := newUiQuestionBuilder(t).
		text("qname1").
		choice("qchoice1", false).
		choice("qchoice2", true).
		choice("qchoice3", false).
		explain(explanation).
		build()
	blocks := []blockInput{ newQuestionBlockInput(question1), newContentBlockInput(contentStr) }
	client.editModule(courseId, newModuleVersion, blocks)

	newModuleVersion2 := db.NewModuleVersion(2, moduleId, 1, "new title2", "new description2")
	question2 := newUiQuestionBuilder(t).
		text("qname2").
		choice("qchoice4", true).
		explain(explanation).
		build()
	blocks2 := []blockInput{ newQuestionBlockInput(question2), newContentBlockInput(contentStr) }
	client.editModule(courseId, newModuleVersion2, blocks2)

	content, err := ctx.db.GetAllContent()
	assert.Nil(t, err)
	assert.Len(t, content, 2)
	contentStrings := []string{content[0].Content, content[1].Content}
	assert.Contains(t, contentStrings, explanation)
	assert.Contains(t, contentStrings, contentStr)
}
