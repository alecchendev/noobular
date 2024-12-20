package internal_test

import (
	"noobular/internal"
	"noobular/internal/db"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNav(t *testing.T) {
	ctx := startServer(t)
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
	ctx := startServer(t)
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
	assert.NotContains(t, body, "Course created")
	assert.NotContains(t, body, "Modules")
}

func TestEditCourse(t *testing.T) {
	ctx := startServer(t)
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	course, modules := sampleCreateCourseInput()
	client.createCourse(course, modules)
	courseId := 1

	body := client.getPageBody("/teacher")
	assert.Contains(t, body, editCourseRoute(courseId))

	newCourse := db.NewCourse(courseId, "new title", "new description", true)
	newModules := []db.ModuleVersion{
		db.NewModuleVersion(-1, 1, 1, "new module title1", "new module description1"),
		db.NewModuleVersion(-1, 2, 1, "new module title2", "new module description2"),
	}
	client.editCourse(newCourse, newModules)

	for _, route := range []string{"/teacher", editCourseRoute(courseId)} {
		body = client.getPageBody(route)
		assert.Contains(t, body, newCourse.Title)
		assert.Contains(t, body, newCourse.Description)
		for _, module := range newModules {
			assert.Contains(t, body, module.Title)
			assert.Contains(t, body, module.Description)
		}
	}

	// Assert a user cannot edit a module for a course that's not theirs
	// even if they put a course that is theirs
	user2 := ctx.createUser()
	client2 := newTestClient(t).login(user2.Id)
	course2, _, _ := client2.initTestCourseN(1, 2)
	module := db.NewModuleVersion(-1, 1, 2, "different module title", "different module description")
	client2.editCourseFail(course2, []db.ModuleVersion{module})
}

func TestPrivateCourse(t *testing.T) {
	ctx := startServer(t)
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	course, modules, _ := client.initTestCourse()

	body := client.getPageBody("/browse")
	assert.Contains(t, body, course.Title)
	assert.Contains(t, body, course.Description)
	for _, module := range modules {
		assert.Contains(t, body, module.Title)
		assert.Contains(t, body, module.Description)
	}

	newCourse := course
	newCourse.Public = false
	client.editCourse(newCourse, modules)

	body = client.getPageBody("/browse")
	assert.NotContains(t, body, course.Title)
	assert.NotContains(t, body, course.Description)
	for _, module := range modules {
		assert.NotContains(t, body, module.Title)
		assert.NotContains(t, body, module.Description)
	}

	client.enrollCourseFail(course.Id)
}

func TestEditModule(t *testing.T) {
	ctx := startServer(t)
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	course, modules, blockInputs := client.initTestCourse()
	courseId := 1

	// Check that if we revisit the edit module page
	// all of our changes are reflected
	for i, module := range modules {
		editModulePageLink := editModuleRoute(courseId, module.ModuleId)
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

	modules = append(modules, db.NewModuleVersion(-1, -1, 1, "new module title3", "new module description3"))
	client.editCourse(course, modules)

	// Course + modules should show up in browse page now that module has blocks
	body := client.getPageBody("/browse")
	assert.Contains(t, body, course.Title)
	assert.Contains(t, body, course.Description)
	for _, module := range modules[:len(modules)-1] {
		assert.Contains(t, body, module.Title)
		assert.Contains(t, body, module.Description)
	}
	assert.NotContains(t, body, modules[len(modules)-1].Title)
	assert.NotContains(t, body, modules[len(modules)-1].Description)

	// Assert a user cannot edit a module for a course that's not theirs
	// even if they put a course that is theirs
	user2 := ctx.createUser()
	client2 := newTestClient(t).login(user2.Id)
	course2, _, _ := client2.initTestCourseN(1, 3)
	client2.editModuleFail(course2.Id, modules[0], blockInputs[0])
	client2.deleteModuleFail(course2.Id, modules[0].ModuleId)
}

func TestAuth(t *testing.T) {
	ctx := startServer(t)
	defer ctx.Close()

	user1 := ctx.createUser()
	client1 := newTestClient(t).login(user1.Id)

	user2 := ctx.createUser()
	client2 := newTestClient(t).login(user2.Id)

	course, modules, _ := client1.initTestCourse()

	body := client1.getPageBody("/teacher")
	assert.Contains(t, body, course.Title)
	assert.Contains(t, body, modules[0].Title)
	assert.Contains(t, body, editCourseRoute(course.Id))
	assert.Contains(t, body, editModuleRoute(course.Id, modules[0].ModuleId))

	body = client2.getPageBody("/teacher")
	assert.NotContains(t, body, course.Title)
	assert.NotContains(t, body, modules[0].Title)
	assert.NotContains(t, body, editCourseRoute(course.Id))
	assert.NotContains(t, body, editModuleRoute(course.Id, modules[0].ModuleId))

	newCourse := db.NewCourse(course.Id, "new title", "new description", true)
	newModules := []db.ModuleVersion{
		db.NewModuleVersion(-1, 1, 1, "new module title1", "new module description1"),
		db.NewModuleVersion(-1, 2, 1, "new module title2", "new module description2"),
	}
	client2.editCourseFail(newCourse, newModules)

	newModuleVersion1 := db.NewModuleVersion(2, modules[0].ModuleId, 1, "new title", "new description")
	contentStr := "qcontent1"
	contentStr2 := "qcontent2"
	blocks := []blockInput{
		newContentBlockInput(contentStr),
		newContentBlockInput(contentStr2),
	}
	client2.editModuleFail(course.Id, newModuleVersion1, blocks)
}

// Test a couple things:
// - If we need the same content for multiple blocks, we should only store it once
// - If we make a new module version, we delete the old version's unique content
//   (even if it's referenced multiple times), but keep the shared content
func TestNoDuplicateContent(t *testing.T) {
	ctx := startServer(t)
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
	ctx := startServer(t)
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
	ctx := startServer(t)
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	course, modules, _ := client.initTestCourse()
	courseId := 1

	client.enrollCourse(courseId)

	body := client.getPageBody(studentCoursePageRoute(courseId))
	assert.Contains(t, body, course.Title)
	for _, module := range modules {
		assert.Equal(t, strings.Count(body, module.Title), 1)
	}

	// If we enroll again in the same course it should not succeed
	client.enrollCourseFail(courseId)
}

// Test module version, i.e. once someone has visited the module, then when you edit it
// and they go back, it's still there
func TestModuleVersioning(t *testing.T) {
	ctx := startServer(t)
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	// Create module with content
	course, modules := sampleCreateCourseInput()
	client.createCourse(course, modules)

	courseId := 1
	moduleId := 1

	// Create one version
	newModuleVersion1 := db.NewModuleVersion(2, moduleId, 1, "new title", "new description")
	contentStr := "qcontent1"
	question := newUiQuestionBuilder().
			text("qname1").
			choice("qchoice1", false).
			choice("qchoice2", true).
			explain("qexplanation1").
			build()
	blocks := []blockInput{
		newContentBlockInput(contentStr),
		newQuestionBlockInput(question),
	}
	client.editModule(courseId, newModuleVersion1, blocks)

	// Visit
	client.enrollCourse(courseId)
	body := client.getPageBody(studentCoursePageRoute(courseId))
	assert.Contains(t, body, takeModulePageRoute(courseId, moduleId))

	// Take module initial page (first block = content)
	body = client.getPageBody(takeModulePageRoute(courseId, moduleId))
	assert.Contains(t, body, contentStr)
	assert.NotContains(t, body, question.QuestionText)
	assert.Contains(t, body, takeModulePieceRoute(courseId, moduleId, 1))

	// Next piece (question)
	body = client.getPageBody(takeModulePieceRoute(courseId, moduleId, 1))
	assert.Contains(t, body, question.QuestionText)
	assert.Contains(t, body, question.Choices[0].ChoiceText)
	assert.Contains(t, body, question.Choices[1].ChoiceText)
	assert.NotContains(t, body, question.Explanation.Content)

	// TODO: Answer question + reveal explanation

	// Edit
	newModuleVersion2 := db.NewModuleVersion(3, moduleId, 2, "new title2", "new description2")
	contentStr2 := "qcontent2"
	question2 := newUiQuestionBuilder().
			text("q2name1").
			choice("q2choice1", false).
			choice("q2choice2", true).
			explain("q2explanation1").
			build()
	blocks2 := []blockInput{
		newQuestionBlockInput(question2),
		newContentBlockInput(contentStr2),
	}
	client.editModule(courseId, newModuleVersion2, blocks2)

	// Visit again, and show old version
	body = client.getPageBody(takeModulePageRoute(courseId, moduleId))
	assert.Contains(t, body, contentStr)
	assert.Contains(t, body, question.QuestionText)
	assert.Contains(t, body, question.Choices[0].ChoiceText)
	assert.Contains(t, body, question.Choices[1].ChoiceText)
	assert.NotContains(t, body, question.Explanation.Content)
}
