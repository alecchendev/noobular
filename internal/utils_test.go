package internal_test

import (
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"noobular/internal"
	"noobular/internal/client"
	"noobular/internal/db"
	"strconv"
	"strings"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/require"
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
	userCount int
}

func (c testContext) Close() {
	c.server.Close()
	c.db.Close()
}

func (c *testContext) createUser() db.User {
	fmt.Println("Creating user:", c.userCount)
	user, err := c.db.CreateUser("test" + strconv.Itoa(c.userCount))
	require.Nil(c.t, err)
	c.userCount += 1
	return user
}

func startServer(t *testing.T) testContext {
	dbClient := db.NewMemoryDbClient()
	server := testServer(dbClient)
	ready := make(chan struct{})
	go func() {
		close(ready)
		server.ListenAndServe()
	}()
	<-ready
	return testContext{t: t, server: server, db: dbClient, userCount: 0}
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

func (c testClient) delete(path string) *http.Response {
	return c.request("DELETE", path, "")
}

func (c testClient) login(userId int64) testClient {
	jwtSecret, _ := hex.DecodeString(testJwtSecretHex)
	cookie, _ := internal.CreateAuthCookie(jwtSecret, userId, false)
	c.session_token = &cookie
	return c
}

func (c testClient) noobClient() client.Client {
	return client.NewClient(c.baseUrl, c.session_token)
}

func (c testClient) getPageBody(path string) string {
	resp := c.get(path)
	require.Equal(c.t, 200, resp.StatusCode)
	return bodyText(c.t, resp)
}

func (c testClient) getPageFail(path string) {
	resp := c.get(path)
	require.NotEqual(c.t, 200, resp.StatusCode)
}

func bodyText(t *testing.T, resp *http.Response) string {
	bodyBytes, err := io.ReadAll(resp.Body)
	require.Nil(t, err)
	return string(bodyBytes)
}

type titleDescInput struct {
	Title string
	Description string
}

func newTitleDescInput(title, desc string) titleDescInput {
	return titleDescInput{title, desc}
}

func sampleCreateCourseInput() (titleDescInput, []titleDescInput) {
	return sampleCreateCourseInputN(1)
}

func sampleCreateCourseInputN(n int) (titleDescInput, []titleDescInput) {
	nStr := strconv.Itoa(n)
	return titleDescInput{ "hello" + nStr, "goodbye" + nStr }, []titleDescInput{
		{ "c" + nStr + "_module title1", "c" + nStr + "_module description1" },
		{ "c" + nStr + "_module title2", "c" + nStr + "_module description2" },
	}
}

const createCourseRoute = "/teacher/course/create"

func NewTestDbCourse(in titleDescInput, public bool) db.Course {
	return db.NewCourse(-1, in.Title, in.Description, public)
}

func newDbCourseAndModules(in titleDescInput, modules []titleDescInput) (db.Course, []db.ModuleVersion) {
	dbCourse := NewTestDbCourse(in, true)
	dbModules := make([]db.ModuleVersion, 0)
	for _, module := range modules {
		dbModules = append(dbModules, db.NewModuleVersion(-1, -1, 0, module.Title, module.Description))
	}
	return dbCourse, dbModules
}

func (c testClient) createCourse(course titleDescInput, moduleInputs []titleDescInput) {
	modules := make([]client.ModuleInit, 0)
	for _, module := range moduleInputs {
		modules = append(modules, client.ModuleInit{
			Title: module.Title,
			Description: module.Description,
		})
	}
	cli := client.NewClient(c.baseUrl, c.session_token)
	resp := cli.CreateCourse(course.Title, course.Description, true, modules)
	require.Equal(c.t, 200, resp.StatusCode)
}

func (c testClient) createCourseFail(course titleDescInput, modules []titleDescInput) {
	dbCourse, dbModules := newDbCourseAndModules(course, modules)
	formData := createOrEditCourseForm(dbCourse, dbModules)
	resp := c.post(createCourseRoute, formData.Encode())
	require.NotEqual(c.t, 200, resp.StatusCode)
}

func editCourseRoute(courseId int) string {
	return fmt.Sprintf("/teacher/course/%d", courseId)
}

func (c testClient) editCourse(course db.Course, modules []db.ModuleVersion) {
	formData := createOrEditCourseForm(course, modules)
	resp := c.put(editCourseRoute(course.Id), formData.Encode())
	require.Equal(c.t, 200, resp.StatusCode)
}

func (c testClient) editCourseFail(course db.Course, modules []db.ModuleVersion) {
	formData := createOrEditCourseForm(course, modules)
	resp := c.put(editCourseRoute(course.Id), formData.Encode())
	require.NotEqual(c.t, 200, resp.StatusCode)
}

func createOrEditCourseForm(course db.Course, modules []db.ModuleVersion) url.Values {
	formData := url.Values{}
	formData.Set("title", course.Title)
	formData.Set("description", course.Description)
	if course.Public {
		formData.Set("public", "on")
	}
	for _, module := range modules {
		formData.Add("module-title[]", module.Title)
		formData.Add("module-id[]", strconv.Itoa(module.ModuleId))
		formData.Add("module-description[]", module.Description)
	}
	return formData
}

func (c testClient) deleteModule(courseId, moduleId int) {
	resp := c.delete(fmt.Sprintf("/teacher/course/%d/module/%d", courseId, moduleId))
	require.Equal(c.t, 200, resp.StatusCode)
}

func (c testClient) deleteModuleFail(courseId, moduleId int) {
	resp := c.delete(fmt.Sprintf("/teacher/course/%d/module/%d", courseId, moduleId))
	require.NotEqual(c.t, 200, resp.StatusCode)
}

type blockInput struct {
	blockType db.BlockType
	block interface{}
}

func newQuestionBlockInput(question internal.UiQuestion) blockInput {
	return blockInput{db.KnowledgePointBlockType, question}
}

func newContentBlockInput(content string) blockInput {
	return blockInput{db.ContentBlockType, db.NewContent(-1, content)}
}

type uiQuestionBuilderChoice struct {
	choiceText string
	isCorrect bool
}

type uiQuestionBuilder struct {
	questionText string
	choices []uiQuestionBuilderChoice
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
	b.choices = append(b.choices, uiQuestionBuilderChoice{choiceText, isCorrect})
	return b
}

func (b uiQuestionBuilder) explain(text string) uiQuestionBuilder {
	b.explanation = text
	return b
}

func (b uiQuestionBuilder) build() internal.UiQuestion {
	choices := make([]db.Choice, 0)
	choiceContents := make([]db.Content, 0)
	for _, choice := range b.choices {
		choiceContents = append(choiceContents, db.NewContent(-1, choice.choiceText))
		choices = append(choices, db.NewChoice(-1, -1, -1, choice.isCorrect))
	}
	return internal.NewUiQuestionEdit(db.NewQuestion(-1, -1, -1), db.NewContent(-1, b.questionText), choices, choiceContents, db.NewContent(-1, b.explanation))
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

func blockInputsToBlocks(inputs []blockInput) []client.Block {
	blocks := []client.Block{}
	for _, input := range inputs {
		switch input.blockType {
		case db.KnowledgePointBlockType:
			question := input.block.(internal.UiQuestion)
			choices := []client.Choice{}
			for _, choice := range question.Choices {
				choices = append(choices, client.Choice{Text: choice.Content.Content, Correct: choice.IsCorrect})
			}
			blocks = append(blocks, client.NewQuestionBlock(question.Content.Content, choices, question.Explanation.Content))
		case db.ContentBlockType:
			content := input.block.(db.Content)
			blocks = append(blocks, client.NewContentBlock(content.Content))
		}
	}
	return blocks
}

func (c testClient) editModule(courseId int64, moduleVersion db.ModuleVersion, blockInputs []blockInput) {
	moduleId := int64(moduleVersion.ModuleId)
	title := moduleVersion.Title
	description := moduleVersion.Description
	blocks := blockInputsToBlocks(blockInputs)
	cli := client.NewClient(c.baseUrl, c.session_token)
	resp := cli.EditModule(int64(courseId), moduleId, title, description, blocks)
	require.Equal(c.t, 200, resp.StatusCode)
}

func (c testClient) editModuleFail(courseId int, moduleVersion db.ModuleVersion, blockInputs []blockInput) {
	moduleId := int64(moduleVersion.ModuleId)
	title := moduleVersion.Title
	description := moduleVersion.Description
	blocks := blockInputsToBlocks(blockInputs)
	cli := client.NewClient(c.baseUrl, c.session_token)
	resp := cli.EditModule(int64(courseId), moduleId, title, description, blocks)
	require.NotEqual(c.t, 200, resp.StatusCode)
}

func prereqForm(prereqs []int) url.Values {
	formData := url.Values{}
	for _, prereq := range prereqs {
		formData.Add("prereqs[]", strconv.Itoa(prereq))
	}
	return formData
}

func setPreReqsRoute(courseId, moduleId int) string {
	return fmt.Sprintf("/teacher/course/%d/module/%d/prereq", courseId, moduleId)
}

func (c testClient) setPrereqs(courseId int, moduleId int, prereqs []int) {
	formData := prereqForm(prereqs)
	resp := c.put(setPreReqsRoute(courseId, moduleId), formData.Encode())
	require.Equal(c.t, 200, resp.StatusCode)
}

func (c testClient) setPrereqsFail(courseId int, moduleId int, prereqs []int) {
	formData := prereqForm(prereqs)
	resp := c.put(setPreReqsRoute(courseId, moduleId), formData.Encode())
	require.NotEqual(c.t, 200, resp.StatusCode)
}

func exportModuleRoute(courseId int, moduleId int) string {
	return fmt.Sprintf("/teacher/course/%d/module/%d/export", courseId, moduleId)
}

// Creates a test course with a module + edits the module to have content.
func (c testClient) initTestCourse() (db.Course, []db.ModuleVersion, [][]blockInput) {
	return c.initTestCourseN(0, 0)
}

func (c testClient) initTestCourseN(courseCount int, moduleCount int) (db.Course, []db.ModuleVersion, [][]blockInput) {
	n := courseCount + 1
	m := moduleCount + 1
	course, initModules := sampleCreateCourseInputN(n)
	c.createCourse(course, initModules)

	courseId := n
	moduleId := m

	body := c.getPageBody("/teacher")
	editModulePageLink := client.EditModuleRoute(int64(courseId), int64(moduleId))
	require.Contains(c.t, body, editModulePageLink)

	body = c.getPageBody(editModulePageLink)
	require.Contains(c.t, body, client.EditModuleRoute(int64(courseId), int64(moduleId)))

	newModules := []db.ModuleVersion{
		db.NewModuleVersion(-1, m, 1, "new title1", "new description1"),
		db.NewModuleVersion(-1, m + 1, 1, "new title2", "new description2"),
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
		c.editModule(int64(courseId), module, blockInputs[i])
	}

	return db.NewCourse(n, course.Title, course.Description, true), newModules, blockInputs
}

func (c testClient) enrollCourse(courseId int) {
	resp := c.post(fmt.Sprintf("/student/course/%d", courseId), "")
	require.Equal(c.t, 200, resp.StatusCode)
}

func (c testClient) enrollCourseFail(courseId int) {
	resp := c.post(fmt.Sprintf("/student/course/%d", courseId), "")
	require.NotEqual(c.t, 200, resp.StatusCode)
}

func (c testClient) completeModule(courseId int, moduleId int) {
	resp := c.put(fmt.Sprintf("/student/course/%d/module/%d/complete", courseId, moduleId), "")
	require.Equal(c.t, 200, resp.StatusCode)
}

func studentCoursePageRoute(courseId int) string {
	return fmt.Sprintf("/student/course/%d", courseId)
}

func takeModulePageRoute(courseId int, moduleId int) string {
	return fmt.Sprintf("/student/course/%d/module/%d", courseId, moduleId)
}

func takeModulePieceRoute(courseId int, moduleId int, blockIdx int) string {
	return fmt.Sprintf("/student/course/%d/module/%d/block/%d/piece", courseId, moduleId, blockIdx)
}

func completeModuleRoute(courseId int, moduleId int) string {
	return fmt.Sprintf("/student/course/%d/module/%d/complete", courseId, moduleId)
}

func nextModulePieceRoute(courseId int, moduleId int, blockIdx int) string {
	return fmt.Sprintf("/student/course/%d/module/%d/block/%d/piece", courseId, moduleId, blockIdx)
}
