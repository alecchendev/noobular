package internal_test

import (
	"fmt"
	"noobular/internal"
	"noobular/internal/db"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

	user := ctx.createUser()

	test := func(t *testing.T, path string, expectedText string) {
		client := newTestClient(t)
		body := client.getPageBody(path)
		assert.Contains(t, body, expectedText)
		assert.Contains(t, body, "Signin")
		assert.Contains(t, body, "Signup")

		client = client.login(user.Id)
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

	_, modules, blockInputs := client.initTestCourse()
	courseId := 1

	// Check that if we revisit the edit module page
	// all of our changes are reflected
	for i, module := range modules {
		editModulePageLink := editModulePageRoute(courseId, module.ModuleId)
		body := client.getPageBody(editModulePageLink)
		assert.Contains(t, body, module.Title)
		assert.Contains(t, body, module.Description)
		for _, block := range blockInputs[i] {
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
	question1 := newUiQuestionBuilder().
		text("qname1").
		choice("qchoice1", false).
		choice("qchoice2", true).
		choice("qchoice3", false).
		explain(explanation).
		build()
	blocks := []blockInput{ newQuestionBlockInput(question1), newContentBlockInput(contentStr) }
	client.editModule(courseId, newModuleVersion, blocks)

	newModuleVersion2 := db.NewModuleVersion(2, moduleId, 1, "new title2", "new description2")
	question2 := newUiQuestionBuilder().
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

func TestStudentCoursePage(t *testing.T) {
	ctx := startServer()
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	course, modules, _ := client.initTestCourse()
	courseId := 1

	client.enrollCourse(courseId)

	body := client.getPageBody(fmt.Sprintf("/student/course/%d", courseId))
	assert.Contains(t, body, course.Title)
	for _, module := range modules {
		assert.Equal(t, strings.Count(body, module.Title), 1)
	}

	// If we enroll again in the same course it should not succeed
	client.enrollCourseFail(courseId)
}
