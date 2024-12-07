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

	course, modules := sampleCreateCourseInput()
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

	// Assert it doesn't show up because it doesn't have any modules with blocks
	body = client.getPageBody("/browse")
	assert.NotContains(t, body, course.Title)
	assert.NotContains(t, body, course.Description)
}

func TestEditCourse(t *testing.T) {
	ctx := startServer()
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	course, modules := sampleCreateCourseInput()
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

// Test a couple things:
// - If we need the same content for multiple blocks, we should only store it once
// - If we make a new module version, we delete the old version's unique content
//   (even if it's referenced multiple times), but keep the shared content
func TestNoDuplicateContent(t *testing.T) {
	ctx := startServer()
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	course, modules := sampleCreateCourseInput()
	client.createCourse(course, modules)

	courseId := 1
	moduleId := 1

	newModuleVersion := db.NewModuleVersion(2, moduleId, 1, "new title", "new description")
	explanation := "qexplanation1"
	contentStr := "qcontent1"
	contentStr2 := "qcontent2" // Shared between blocks within version
	question1 := newUiQuestionBuilder().
		text("qname1").
		choice("qchoice1", false).
		choice("qchoice2", true).
		choice("qchoice3", false).
		explain(explanation).
		build()
	question1_2 := newUiQuestionBuilder().
		text("qname1").
		choice("qchoice1", true).
		explain(contentStr2).
		build()
	blocks := []blockInput{
		newQuestionBlockInput(question1),
		newQuestionBlockInput(question1_2),
		newContentBlockInput(contentStr),
		newContentBlockInput(contentStr2),
	}
	client.editModule(courseId, newModuleVersion, blocks)

	// Assert contentStr2 is not duplicated (shared between explanation and content block)
	{
		content, err := ctx.db.GetAllContent()
		assert.Nil(t, err)
		assert.Len(t, content, 3)
		contentStrings := []string{content[0].Content, content[1].Content, content[2].Content}
		assert.Contains(t, contentStrings, explanation)
		assert.Contains(t, contentStrings, contentStr)
		assert.Contains(t, contentStrings, contentStr2)
	}

	newModuleVersion2 := db.NewModuleVersion(2, moduleId, 1, "new title2", "new description2")
	question2 := newUiQuestionBuilder().
		text("qname2").
		choice("qchoice4", true).
		explain(explanation).
		build()
	blocks2 := []blockInput{ newQuestionBlockInput(question2), newContentBlockInput(contentStr) }
	client.editModule(courseId, newModuleVersion2, blocks2)

	// Assert contentStr is not duplicated
	// Assert contentStr2 is deleted
	content, err := ctx.db.GetAllContent()
	assert.Nil(t, err)
	assert.Len(t, content, 2)
	contentStrings := []string{content[0].Content, content[1].Content}
	assert.Contains(t, contentStrings, explanation)
	assert.Contains(t, contentStrings, contentStr)
}

// Test that if we delete a module, content unique to that module is deleted,
// but content shared with other modules is not deleted
func TestDeleteModuleSharedContent(t *testing.T) {
	ctx := startServer()
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	// Create a course with a module with one unique content, and one shared content
	course, modules := sampleCreateCourseInput()
	client.createCourse(course, modules)

	courseId1 := 1
	moduleId1 := 1

	newModuleVersion1 := db.NewModuleVersion(2, moduleId1, 1, "new title", "new description")
	contentStr := "qcontent1"
	contentStr2 := "qcontent2"
	blocks := []blockInput{
		newContentBlockInput(contentStr),
		newContentBlockInput(contentStr2),
	}
	client.editModule(courseId1, newModuleVersion1, blocks)

	// Create a course with a module with one shared content
	client.createCourse(course, modules)

	courseId2 := 2
	moduleId2 := 2

	newModuleVersion2 := db.NewModuleVersion(2, moduleId2, 1, "new title", "new description")
	blocks = []blockInput{
		newContentBlockInput(contentStr),
	}
	client.editModule(courseId2, newModuleVersion2, blocks)

	// Delete first courses module
	client.deleteModule(courseId1, moduleId1)

	// Assert shared content stays
	// Assert unique content is deleted
	content, err := ctx.db.GetAllContent()
	assert.Nil(t, err)
	assert.Len(t, content, 1)
	assert.Contains(t, content[0].Content, contentStr)
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
