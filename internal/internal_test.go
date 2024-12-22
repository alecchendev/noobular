package internal_test

import (
	"noobular/internal"
	"noobular/internal/db"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
		require.Contains(t, body, expectedText)
		require.Contains(t, body, "Signin")
		require.Contains(t, body, "Signup")

		client = client.login(user.Id)
		body = client.getPageBody(path)
		require.Contains(t, body, expectedText)
		require.Contains(t, body, "Logout")
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
	require.Contains(t, body, createCourseRoute)
	require.NotContains(t, body, course.Title)

	client.createCourse(course, modules)

	body = client.getPageBody("/teacher")
	require.Contains(t, body, course.Title)
	require.Contains(t, body, course.Description)
	for _, module := range modules {
		require.Contains(t, body, module.Title)
		require.Contains(t, body, module.Description)
	}

	// require it doesn't show up because it doesn't have any modules with blocks
	body = client.getPageBody("/browse")
	require.NotContains(t, body, course.Title)
	require.NotContains(t, body, course.Description)
	require.NotContains(t, body, "Course created")
	require.NotContains(t, body, "Modules")
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
	require.Contains(t, body, editCourseRoute(courseId))

	newCourse := db.NewCourse(courseId, "new title", "new description", true)
	newModules := []db.ModuleVersion{
		db.NewModuleVersion(-1, 1, 1, "new module title1", "new module description1"),
		db.NewModuleVersion(-1, 2, 1, "new module title2", "new module description2"),
	}
	client.editCourse(newCourse, newModules)

	for _, route := range []string{"/teacher", editCourseRoute(courseId)} {
		body = client.getPageBody(route)
		require.Contains(t, body, newCourse.Title)
		require.Contains(t, body, newCourse.Description)
		for _, module := range newModules {
			require.Contains(t, body, module.Title)
			require.Contains(t, body, module.Description)
		}
	}

	// require a user cannot edit a module for a course that's not theirs
	// even if they put a course that is theirs
	user2 := ctx.createUser()
	client2 := newTestClient(t).login(user2.Id)
	course2, _, _ := client2.initTestCourseN(1, 2)
	module := db.NewModuleVersion(-1, 1, 2, "different module title", "different module description")
	client2.editCourseFail(course2, []db.ModuleVersion{module})
}

func TestCreateEditCourseInputValidation(t *testing.T) {
	ctx := startServer(t)
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	course, modules := sampleCreateCourseInput()

	emptyTitle := newTitleDescInput("", "description")
	emptyDescription := newTitleDescInput("title", "")
	tooLongTitle := newTitleDescInput(strings.Repeat("a", internal.TitleMaxLength + 1), "description")
	tooLongDescription := newTitleDescInput("title", strings.Repeat("a", internal.DescriptionMaxLength + 1))
	emptyTitleModules := []titleDescInput{emptyTitle}
	emptyDescriptionModules := []titleDescInput{emptyDescription}
	tooLongTitleModules := []titleDescInput{tooLongTitle}
	tooLongDescriptionModules := []titleDescInput{tooLongDescription}

	// Create
	client.createCourseFail(emptyTitle, modules)
	client.createCourseFail(emptyDescription, modules)
	client.createCourseFail(tooLongTitle, modules)
	client.createCourseFail(tooLongDescription, modules)
	client.createCourseFail(course, emptyTitleModules)
	client.createCourseFail(course, emptyDescriptionModules)
	client.createCourseFail(course, tooLongTitleModules)
	client.createCourseFail(course, tooLongDescriptionModules)
	tooManyModules := make([]titleDescInput, internal.MaxModules + 1)
	for i := range tooManyModules {
		tooManyModules[i] = newTitleDescInput("title", "description")
	}
	client.createCourseFail(course, tooManyModules)

	// Edit
	client.createCourse(course, modules)
	courseId := 1
	newCourse := db.NewCourse(courseId, "new title", "new description", true)
	newModules := []db.ModuleVersion{
		db.NewModuleVersion(-1, 1, 1, "new module title1", "new module description1"),
		db.NewModuleVersion(-1, 2, 1, "new module title2", "new module description2"),
	}
	client.editCourse(newCourse, newModules)

	dbCourse := func(in titleDescInput) db.Course {
		return db.NewCourse(courseId, in.Title, in.Description, true)
	}
	dbMdoules := func(in []titleDescInput) []db.ModuleVersion {
		out := make([]db.ModuleVersion, len(in))
		for i, module := range in {
			out[i] = db.NewModuleVersion(-1, i + 1, 1, module.Title, module.Description)
		}
		return out
	}
	client.editCourseFail(dbCourse(emptyTitle), newModules)
	client.editCourseFail(dbCourse(emptyDescription), newModules)
	client.editCourseFail(dbCourse(tooLongTitle), newModules)
	client.editCourseFail(dbCourse(tooLongDescription), newModules)
	client.editCourseFail(newCourse, dbMdoules(emptyTitleModules))
	client.editCourseFail(newCourse, dbMdoules(emptyDescriptionModules))
	client.editCourseFail(newCourse, dbMdoules(tooLongTitleModules))
	client.editCourseFail(newCourse, dbMdoules(tooLongDescriptionModules))
	client.editCourseFail(newCourse, dbMdoules(tooManyModules))
}

func TestPrivateCourse(t *testing.T) {
	ctx := startServer(t)
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	course, modules, _ := client.initTestCourse()

	body := client.getPageBody("/browse")
	require.Contains(t, body, course.Title)
	require.Contains(t, body, course.Description)
	for _, module := range modules {
		require.Contains(t, body, module.Title)
		require.Contains(t, body, module.Description)
	}

	newCourse := course
	newCourse.Public = false
	client.editCourse(newCourse, modules)

	body = client.getPageBody("/browse")
	require.NotContains(t, body, course.Title)
	require.NotContains(t, body, course.Description)
	for _, module := range modules {
		require.NotContains(t, body, module.Title)
		require.NotContains(t, body, module.Description)
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
		require.Contains(t, body, module.Title)
		require.Contains(t, body, module.Description)
		for _, block := range blockInputs[i] {
			switch block.blockType {
			case db.QuestionBlockType:
				question := block.block.(internal.UiQuestion)
				require.Contains(t, body, question.QuestionText)
				require.Contains(t, body, question.Explanation.Content)
				for _, choice := range question.Choices {
					require.Contains(t, body, choice.ChoiceText)
				}
			case db.ContentBlockType:
				content := block.block.(db.Content)
				require.Contains(t, body, content.Content)
			}
		}
	}

	modules = append(modules, db.NewModuleVersion(-1, -1, 1, "new module title3", "new module description3"))
	client.editCourse(course, modules)

	// Course + modules should show up in browse page now that module has blocks
	body := client.getPageBody("/browse")
	require.Contains(t, body, course.Title)
	require.Contains(t, body, course.Description)
	for _, module := range modules[:len(modules)-1] {
		require.Contains(t, body, module.Title)
		require.Contains(t, body, module.Description)
	}
	require.NotContains(t, body, modules[len(modules)-1].Title)
	require.NotContains(t, body, modules[len(modules)-1].Description)

	// require a user cannot edit a module for a course that's not theirs
	// even if they put a course that is theirs
	user2 := ctx.createUser()
	client2 := newTestClient(t).login(user2.Id)
	course2, _, _ := client2.initTestCourseN(1, 3)
	client2.editModuleFail(course2.Id, modules[0], blockInputs[0])
	client2.deleteModuleFail(course2.Id, modules[0].ModuleId)
}

func TestEditModuleInputValidation(t *testing.T) {
	ctx := startServer(t)
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	course, modules, blocks := client.initTestCourse()
	module := modules[0]
	module.Id = 1
	blockInputs := blocks[0]

	emptyTitle := newTitleDescInput("", "description")
	emptyDescription := newTitleDescInput("title", "")
	tooLongTitle := newTitleDescInput(strings.Repeat("a", internal.TitleMaxLength + 1), "description")
	tooLongDescription := newTitleDescInput("title", strings.Repeat("a", internal.DescriptionMaxLength + 1))

	dbModule := func(in titleDescInput) db.ModuleVersion {
		return db.NewModuleVersion(2, int(module.Id), 2, in.Title, in.Description)
	}
	client.editModuleFail(course.Id, dbModule(emptyTitle), blockInputs)
	client.editModuleFail(course.Id, dbModule(emptyDescription), blockInputs)
	client.editModuleFail(course.Id, dbModule(tooLongTitle), blockInputs)
	client.editModuleFail(course.Id, dbModule(tooLongDescription), blockInputs)
	client.editModuleFail(course.Id, module, []blockInput{
		newQuestionBlockInput(newUiQuestionBuilder().text("").choice("choice", true).build()),
	})
	client.editModuleFail(course.Id, module, []blockInput{
		newQuestionBlockInput(newUiQuestionBuilder().text(strings.Repeat("a", internal.MaxQuestionLength + 1)).choice("choice", true).build()),
	})
	client.editModuleFail(course.Id, module, []blockInput{
		newQuestionBlockInput(newUiQuestionBuilder().text("question").build()),
	})
	client.editModuleFail(course.Id, module, []blockInput{
		newQuestionBlockInput(newUiQuestionBuilder().text("question").choice("", true).build()),
	})
	client.editModuleFail(course.Id, module, []blockInput{
		newQuestionBlockInput(newUiQuestionBuilder().text("question").choice(strings.Repeat("a", internal.MaxChoiceLength + 1), true).build()),
	})
	client.editModuleFail(course.Id, module, []blockInput{
		newQuestionBlockInput(newUiQuestionBuilder().text("question").choice("choice", false).build()),
	})
	tooManyChoices := newUiQuestionBuilder().text("question")
	for i := 0; i < internal.MaxChoices; i++ {
		tooManyChoices.choice("choice", false)
	}
	tooManyChoices.choice("choice", true)
	client.editModuleFail(course.Id, module, []blockInput{
		newQuestionBlockInput(tooManyChoices.build()),
	})
	client.editModuleFail(course.Id, module, []blockInput{
		newContentBlockInput(""),
	})
	client.editModuleFail(course.Id, module, []blockInput{
		newContentBlockInput(strings.Repeat("a", internal.MaxContentLength + 1)),
	})
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
	require.Contains(t, body, course.Title)
	require.Contains(t, body, modules[0].Title)
	require.Contains(t, body, editCourseRoute(course.Id))
	require.Contains(t, body, editModuleRoute(course.Id, modules[0].ModuleId))

	body = client2.getPageBody("/teacher")
	require.NotContains(t, body, course.Title)
	require.NotContains(t, body, modules[0].Title)
	require.NotContains(t, body, editCourseRoute(course.Id))
	require.NotContains(t, body, editModuleRoute(course.Id, modules[0].ModuleId))

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

	// require contentStr2 is not duplicated (shared between explanation and content block)
	{
		content, err := ctx.db.GetAllContent()
		require.Nil(t, err)
		require.Len(t, content, 3)
		contentStrings := []string{content[0].Content, content[1].Content, content[2].Content}
		require.Contains(t, contentStrings, explanation)
		require.Contains(t, contentStrings, contentStr)
		require.Contains(t, contentStrings, contentStr2)
	}

	newModuleVersion2 := db.NewModuleVersion(2, moduleId, 1, "new title2", "new description2")
	question2 := newUiQuestionBuilder().
		text("qname2").
		choice("qchoice4", true).
		explain(explanation).
		build()
	blocks2 := []blockInput{ newQuestionBlockInput(question2), newContentBlockInput(contentStr) }
	client.editModule(courseId, newModuleVersion2, blocks2)

	// require contentStr is not duplicated
	// require contentStr2 is deleted
	content, err := ctx.db.GetAllContent()
	require.Nil(t, err)
	require.Len(t, content, 2)
	contentStrings := []string{content[0].Content, content[1].Content}
	require.Contains(t, contentStrings, explanation)
	require.Contains(t, contentStrings, contentStr)
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

	// require shared content stays
	// require unique content is deleted
	content, err := ctx.db.GetAllContent()
	require.Nil(t, err)
	require.Len(t, content, 1)
	require.Contains(t, content[0].Content, contentStr)
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
	require.Contains(t, body, course.Title)
	for _, module := range modules {
		require.Equal(t, strings.Count(body, module.Title), 1)
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
	require.Contains(t, body, takeModulePageRoute(courseId, moduleId))

	// Take module initial page (first block = content)
	body = client.getPageBody(takeModulePageRoute(courseId, moduleId))
	require.Contains(t, body, contentStr)
	require.NotContains(t, body, question.QuestionText)
	require.Contains(t, body, takeModulePieceRoute(courseId, moduleId, 1))

	// Next piece (question)
	body = client.getPageBody(takeModulePieceRoute(courseId, moduleId, 1))
	require.Contains(t, body, question.QuestionText)
	require.Contains(t, body, question.Choices[0].ChoiceText)
	require.Contains(t, body, question.Choices[1].ChoiceText)
	require.NotContains(t, body, question.Explanation.Content)

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
	require.Contains(t, body, contentStr)
	require.Contains(t, body, question.QuestionText)
	require.Contains(t, body, question.Choices[0].ChoiceText)
	require.Contains(t, body, question.Choices[1].ChoiceText)
	require.NotContains(t, body, question.Explanation.Content)
}
