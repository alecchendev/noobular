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
	return internal.NewServer(dbClient, renderer, webAuthn, jwtSecret, port, internal.Local)
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
	cookie, _ := internal.CreateAuthCookie(jwtSecret, userId, false)
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
	questionText string
	choices []db.Choice
	explanation string
}

func newUiQuestionBuilder() uiQuestionBuilder {
	return uiQuestionBuilder{}
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

func newTestUiQuestion(moduleId int64, questionNumber int) internal.UiQuestion {
	mid := strconv.Itoa(int(moduleId))
	qnum := strconv.Itoa(questionNumber)
	return newUiQuestionBuilder().
		text("m" + mid + "_qname" + qnum).
		choice("q" + qnum + "_choice1", false).
		choice("q" + qnum + "_choice2", true).
		choice("q" + qnum + "_choice3", false).
		explain("q" + qnum + "_explanation").
		build()
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

// Creates a test course with a module + edits the module to have content.
func (c testClient) initTestCourse() (db.Course, []db.ModuleVersion, [][]blockInput) {
	course := db.NewCourse(-1, "hello", "goodbye")
	initModules := []db.ModuleVersion{
		db.NewModuleVersion(-1, -1, 0, "module title1", "module description1"),
		db.NewModuleVersion(-1, -1, 0, "module title2", "module description2"),
	}
	c.createCourse(course, initModules)

	courseId := 1
	moduleId := 1

	body := c.getPageBody("/teacher")
	editModulePageLink := editModulePageRoute(courseId, moduleId)
	assert.Contains(c.t, body, editModulePageLink)

	body = c.getPageBody(editModulePageLink)
	assert.Contains(c.t, body, editModuleRoute(courseId, moduleId))

	newModules := []db.ModuleVersion{
		db.NewModuleVersion(-1, 1, 1, "new title1", "new description1"),
		db.NewModuleVersion(-1, 2, 1, "new title2", "new description2"),
	}

	blockInputs := make([][]blockInput, len(newModules))
	for i, module := range newModules {
		mid := strconv.Itoa(int(module.ModuleId))
		blockInputs[i] = []blockInput{
			newQuestionBlockInput(newTestUiQuestion(int64(module.ModuleId), 1)),
			newContentBlockInput("m" + mid + "_content1"),
			newQuestionBlockInput(newTestUiQuestion(int64(module.ModuleId), 2)),
			newContentBlockInput("m" + mid + "_content2"),
		}
		c.editModule(courseId, module, blockInputs[i])
	}

	return course, newModules, blockInputs
}

func (c testClient) enrollCourse(courseId int) {
	resp := c.post(fmt.Sprintf("/student/course/%d", courseId), "")
	assert.Equal(c.t, 200, resp.StatusCode)
}

func (c testClient) enrollCourseFail(courseId int) {
	resp := c.post(fmt.Sprintf("/student/course/%d", courseId), "")
	assert.NotEqual(c.t, 200, resp.StatusCode)
}

