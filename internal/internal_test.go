package internal_test

import (
	"bufio"
	"database/sql"
	"fmt"
	"net/http"
	"noobular/internal"
	noob_client "noobular/internal/client"
	"noobular/internal/db"
	"regexp"
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
	client := newTestNoobClient(user.Id)

	courseTitle := "course"
	courseDescription := "description"
	modules := []noob_client.ModuleInit{
		{Title: "module1", Description: "description1"},
		{Title: "module2", Description: "description2"},
	}
	resp := client.CreateCourse(courseTitle, courseDescription, false, modules)
	require.Equal(t, 200, resp.StatusCode)
	courseId := int64(1)

	body := getPageBody(t, client, "/teacher")
	require.Contains(t, body, noob_client.EditCourseRoute(courseId))

	newCourseTitle := "new course title"
	newCourseDescription := "new course description"
	newModules := []noob_client.ModuleUpdate{
		{Id: 1, Title: "new module title1", Description: "new module description1"},
		{Id: 2, Title: "new module title2", Description: "new module description2"},
	}
	resp = client.EditCourse(courseId, newCourseTitle, newCourseDescription, false, newModules)
	require.Equal(t, 200, resp.StatusCode)

	for _, route := range []string{"/teacher", noob_client.EditCourseRoute(courseId)} {
		body = getPageBody(t, client, route)
		require.Contains(t, body, newCourseTitle)
		require.Contains(t, body, newCourseDescription)
		for _, module := range newModules {
			require.Contains(t, body, module.Title)
			require.Contains(t, body, module.Description)
		}
	}

	// require a user cannot edit a module for a course that's not theirs
	// even if they put a course that is theirs
	user2 := ctx.createUser()
	client2 := newTestNoobClient(user2.Id)
	resp = client2.CreateCourse(courseTitle, courseDescription, false, modules)
	courseId2 := int64(2)
	resp = client2.EditCourse(courseId2, newCourseTitle, newCourseDescription, false, newModules)
	require.NotEqual(t, 200, resp.StatusCode)
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
	client := newTestNoobClient(user.Id)

	course, module := createSampleCourse(t, client, 1, 1)
	courseId := int64(course.Id)
	moduleUpdate := noob_client.ModuleUpdate{
		Id: 1, Title: module.Title, Description: module.Description,
	}
	populateSampleModule(t, client, moduleUpdate, 1, 1)

	body := getPageBody(t, client, "/browse")
	require.Contains(t, body, course.Title)
	require.Contains(t, body, course.Description)
	require.Contains(t, body, moduleUpdate.Title)
	require.Contains(t, body, moduleUpdate.Description)

	moduleUpdates := []noob_client.ModuleUpdate{
		{Id: 1, Title: "new module title1", Description: "new module description1"},
	}
	resp := client.EditCourse(courseId, course.Title, course.Description, false, moduleUpdates)

	body = getPageBody(t, client, "/browse")
	require.NotContains(t, body, course.Title)
	require.NotContains(t, body, course.Description)
	for _, module := range moduleUpdates {
		require.NotContains(t, body, module.Title)
		require.NotContains(t, body, module.Description)
	}

	resp = client.EnrollCourse(courseId)
	require.NotEqual(t, 200, resp.StatusCode)
}

func TestEditModule(t *testing.T) {
	ctx := startServer(t)
	defer ctx.Close()

	user := ctx.createUser()
	nClient := newTestNoobClient(user.Id)

	courseTitle := "course"
	courseDescription := "description"
	var resp *http.Response
	module := noob_client.ModuleInit{
		Title: "module", Description: "description",
	}
	resp = nClient.CreateCourse(courseTitle, courseDescription, false, []noob_client.ModuleInit{module})
	require.Equal(t, 200, resp.StatusCode)
	courseId := int64(1)
	moduleId := int64(1)

	question1 := noob_client.QuestionBlock {
		Text: "kp1 question",
		Choices: []noob_client.Choice{
			{Text: "kp1 choice1", Correct: false},
			{Text: "kp1 choice2", Correct: true},
		},
		Explanation: "kp1 explanation",
	}
	resp = nClient.CreateKnowledgePoint(courseId, "kp1", []noob_client.QuestionBlock{question1})
	require.Equal(t, 200, resp.StatusCode)
	kpId1 := int64(1)

	question2 := noob_client.QuestionBlock {
		Text: "kp2 question",
		Choices: []noob_client.Choice{
			{Text: "kp2 choice1", Correct: false},
			{Text: "kp2 choice2", Correct: true},
		},
		Explanation: "kp2 explanation",
	}
	resp = nClient.CreateKnowledgePoint(courseId, "kp2", []noob_client.QuestionBlock{question2})
	require.Equal(t, 200, resp.StatusCode)
	kpId2 := int64(2)

	blocks := []noob_client.Block{
		noob_client.NewContentBlock("content1"),
		noob_client.NewKnowledgePointBlock(kpId1),
		noob_client.NewContentBlock("content2"),
		noob_client.NewKnowledgePointBlock(kpId2),
	}
	resp = nClient.EditModule(courseId, moduleId, module.Title, module.Description, blocks)
	require.Equal(t, 200, resp.StatusCode)

	checkModule := func(moduleId int64, module noob_client.ModuleInit, blocks []noob_client.Block) {
		editModulePageLink := noob_client.EditModuleRoute(courseId, moduleId)
		body := getPageBody(t, nClient, editModulePageLink)
		require.Contains(t, body, module.Title)
		require.Contains(t, body, module.Description)
		for _, block := range blocks {
			switch block.BlockType {
			case noob_client.KnowledgePointBlockType:
				kpId := block.KnowledgePoint.Id
				re := regexp.MustCompile(fmt.Sprintf(`value="%d"\s+selected`, kpId))
				require.Regexp(t, re, body)
			case noob_client.ContentBlockType:
				require.Contains(t, body, block.Content.Text)
			}
		}
	}

	// Check that if we revisit the edit module page
	// all of our changes are reflected
	checkModule(moduleId, module, blocks)

	// Add new module
	module2 := noob_client.ModuleInit{
		Title: "module2", Description: "description2",
	}
	resp = nClient.CreateCourse(courseTitle, courseDescription, false, []noob_client.ModuleInit{module, module2})
	require.Equal(t, 200, resp.StatusCode)

	// Course + modules should show up in browse page now that module has blocks
	body := getPageBody(t, nClient, "/browse")
	require.Contains(t, body, courseTitle)
	require.Contains(t, body, courseDescription)
	require.Contains(t, body, module.Title)
	require.Contains(t, body, module.Description)
	require.NotContains(t, body, module2.Title)
	require.NotContains(t, body, module2.Description)

	// Edit again and make sure creating new module version,
	// with some things edited some things not, works.
	resp = nClient.EditModule(courseId, moduleId, module.Title, module.Description, blocks)
	require.Equal(t, 200, resp.StatusCode)
	checkModule(moduleId, module, blocks)

	// require a user cannot edit a module for a course that's not theirs
	// even if they put a course that is theirs
	user2 := ctx.createUser()
	nClient2 := newTestNoobClient(user2.Id)
	course2Title := "course2"
	course2Description := "description2"
	resp = nClient2.CreateCourse(course2Title, course2Description, false, []noob_client.ModuleInit{module})
	require.Equal(t, 200, resp.StatusCode)
	course2Id := int64(2)
	resp = nClient2.EditModule(course2Id, moduleId, module.Title, module.Description, blocks)
	require.NotEqual(t, 200, resp.StatusCode)

	resp = nClient2.GetPage(noob_client.ExportModuleRoute(course2Id, moduleId))
	require.NotEqual(t, 200, resp.StatusCode)
}

// Creates a course with an empty module
func createSampleCourse(t *testing.T, client noob_client.Client, courseId int64, moduleId int64) (db.Course, db.ModuleVersion) {
	prefix := fmt.Sprintf("module%d", moduleId)
	module := noob_client.ModuleInit{
		Title: fmt.Sprintf("%s title", prefix),
		Description: fmt.Sprintf("%s description", prefix),
	}
	resp := client.CreateCourse(fmt.Sprintf("course%d", courseId), fmt.Sprintf("description%d", courseId), true, []noob_client.ModuleInit{module})
	require.Equal(t, 200, resp.StatusCode)
	course := db.NewCourse(int(courseId), module.Title, module.Description, true)
	moduleVersionId := int64(1)
	moduleVersion := db.NewModuleVersion(moduleVersionId, int(moduleId), 1, module.Title, module.Description)
	return course, moduleVersion
}

func populateSampleModule(t *testing.T, client noob_client.Client, moduleUpdate noob_client.ModuleUpdate, courseId, kpId int64) []noob_client.Block {
	createSampleKnowledgePoint(t, client, courseId, kpId)
	blocks := []noob_client.Block{
		noob_client.NewKnowledgePointBlock(kpId),
		noob_client.NewContentBlock(fmt.Sprintf("module%d content", moduleUpdate.Id)),
	}
	resp := client.EditModule(courseId, moduleUpdate.Id, moduleUpdate.Title, moduleUpdate.Description, blocks)
	require.Equal(t, 200, resp.StatusCode)
	return blocks
}

func createSampleKnowledgePoint(t *testing.T, client noob_client.Client, courseId, kpId int64) int64 {
	prefix := fmt.Sprintf("kp%d", kpId)
	question := noob_client.QuestionBlock {
		Text: fmt.Sprintf("%s question", prefix),
		Choices: []noob_client.Choice{
			{Text: fmt.Sprintf("%s choice1", prefix), Correct: false},
			{Text: fmt.Sprintf("%s choice2", prefix), Correct: true},
		},
		Explanation: fmt.Sprintf("%s explanation", prefix),
	}
	resp := client.CreateKnowledgePoint(courseId, prefix, []noob_client.QuestionBlock{question})
	require.Equal(t, 200, resp.StatusCode)
	return kpId
}

func TestInputValidationEditModule(t *testing.T) {
	ctx := startServer(t)
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestNoobClient(user.Id)

	course, module := createSampleCourse(t, client, 1, 1)

	emptyTitle := newTitleDescInput("", "description")
	emptyDescription := newTitleDescInput("title", "")
	tooLongTitle := newTitleDescInput(strings.Repeat("a", internal.TitleMaxLength + 1), "description")
	tooLongDescription := newTitleDescInput("title", strings.Repeat("a", internal.DescriptionMaxLength + 1))

	resp := client.EditModule(int64(course.Id), int64(module.ModuleId), emptyTitle.Title, emptyTitle.Description, []noob_client.Block{})
	require.NotEqual(t, 200, resp.StatusCode)

	resp = client.EditModule(int64(course.Id), int64(module.ModuleId), emptyDescription.Title, emptyDescription.Description, []noob_client.Block{})
	require.NotEqual(t, 200, resp.StatusCode)

	resp = client.EditModule(int64(course.Id), int64(module.ModuleId), tooLongTitle.Title, tooLongTitle.Description, []noob_client.Block{})
	require.NotEqual(t, 200, resp.StatusCode)

	resp = client.EditModule(int64(course.Id), int64(module.ModuleId), tooLongDescription.Title, tooLongDescription.Description, []noob_client.Block{})
	require.NotEqual(t, 200, resp.StatusCode)

	resp = client.EditModule(int64(course.Id), int64(module.ModuleId), "title", "description", []noob_client.Block{
		noob_client.NewContentBlock(""),
	})
	require.NotEqual(t, 200, resp.StatusCode)

	resp = client.EditModule(int64(course.Id), int64(module.ModuleId), "title", "description", []noob_client.Block{
		noob_client.NewContentBlock(strings.Repeat("a", internal.MaxContentLength + 1)),
	})
	require.NotEqual(t, 200, resp.StatusCode)
}

func TestAuth(t *testing.T) {
	ctx := startServer(t)
	defer ctx.Close()

	user1 := ctx.createUser()
	client1 := newTestNoobClient(user1.Id)

	user2 := ctx.createUser()
	client2 := newTestNoobClient(user2.Id)

	course, module := createSampleCourse(t, client1, 1, 1)

	body := getPageBody(t, client1, "/teacher")
	require.Contains(t, body, course.Title)
	require.Contains(t, body, module.Title)
	require.Contains(t, body, editCourseRoute(course.Id))
	require.Contains(t, body, noob_client.EditModuleRoute(int64(course.Id), int64(module.ModuleId)))

	body = getPageBody(t, client2, "/teacher")
	require.NotContains(t, body, course.Title)
	require.NotContains(t, body, module.Title)
	require.NotContains(t, body, editCourseRoute(course.Id))
	require.NotContains(t, body, noob_client.EditModuleRoute(int64(course.Id), int64(module.ModuleId)))

	newCourse := db.NewCourse(course.Id, "new title", "new description", true)
	newModules := []noob_client.ModuleUpdate{
		{Id: 1, Title: "new module title1", Description: "new module description1"},
	}
	resp := client2.EditCourse(int64(course.Id), newCourse.Title, newCourse.Description, false, newModules)
	require.NotEqual(t, 200, resp.StatusCode)

	resp = client2.EditModule(int64(course.Id), int64(module.ModuleId), module.Title, module.Description, []noob_client.Block{
		noob_client.NewContentBlock("content"),
	})
	require.NotEqual(t, 200, resp.StatusCode)
}

// Test a couple things:
// - If we need the same content for multiple blocks, we should only store it once
// - If we make a new module version, we delete the old version's unique content
//   (even if it's referenced multiple times), but keep the shared content
// func TestNoDuplicateContent(t *testing.T) {
// 	ctx := startServer(t)
// 	defer ctx.Close()
//
// 	user := ctx.createUser()
// 	client := newTestClient(t).login(user.Id)
//
// 	course, modules := sampleCreateCourseInput()
// 	client.createCourse(course, modules)
//
// 	courseId := 1
// 	moduleId := 1
//
// 	newModuleVersion := db.NewModuleVersion(2, moduleId, 1, "new title", "new description")
// 	explanation := "qexplanation1"
// 	contentStr := "qcontent1"
// 	contentStr2 := "qcontent2" // Shared between blocks within version
// 	question1 := newUiQuestionBuilder().
// 		text("qname1").
// 		choice("qchoice1", false).
// 		choice("qchoice2", true).
// 		choice("qchoice3", false).
// 		explain(explanation).
// 		build()
// 	question1_2 := newUiQuestionBuilder().
// 		text("qname1").
// 		choice("qchoice1", true).
// 		explain(contentStr2).
// 		build()
// 	blocks := []blockInput{
// 		newQuestionBlockInput(question1),
// 		newQuestionBlockInput(question1_2),
// 		newContentBlockInput(contentStr),
// 		newContentBlockInput(contentStr2),
// 	}
// 	client.editModule(int64(courseId), newModuleVersion, blocks)
//
// 	// require contentStr2 is not duplicated (shared between explanation and content block)
// 	// require question name/choice text is not duplicated
// 	{
// 		contentExplanationCount := 3
// 		questionChoiceContentCount := 4
// 		content, err := ctx.db.GetAllContent()
// 		require.Nil(t, err)
// 		require.Len(t, content, contentExplanationCount + questionChoiceContentCount)
// 		contentStrings := []string{}
// 		for _, c := range content {
// 			contentStrings = append(contentStrings, c.Content)
// 		}
// 		require.Contains(t, contentStrings, explanation)
// 		require.Contains(t, contentStrings, contentStr)
// 		require.Contains(t, contentStrings, contentStr2)
// 	}
//
// 	newModuleVersion2 := db.NewModuleVersion(2, moduleId, 1, "new title2", "new description2")
// 	question2 := newUiQuestionBuilder().
// 		text("qname2").
// 		choice("qchoice4", true).
// 		explain(explanation).
// 		build()
// 	blocks2 := []blockInput{ newQuestionBlockInput(question2), newContentBlockInput(contentStr) }
// 	client.editModule(int64(courseId), newModuleVersion2, blocks2)
//
// 	// require contentStr is not duplicated
// 	// require contentStr2 is deleted
// 	// question 1 content is deleted
// 	questionChoiceContentCount := 2
// 	explanationContentCount := 2
// 	content, err := ctx.db.GetAllContent()
// 	require.Nil(t, err)
// 	require.Len(t, content, questionChoiceContentCount + explanationContentCount)
// 	contentStrings := []string{}
// 	for _, c := range content {
// 		contentStrings = append(contentStrings, c.Content)
// 	}
// 	require.Contains(t, contentStrings, explanation)
// 	require.Contains(t, contentStrings, contentStr)
// }

// Test that if we delete a module, content unique to that module is deleted,
// but content shared with other modules is not deleted
// func TestDeleteModuleSharedContent(t *testing.T) {
// 	ctx := startServer(t)
// 	defer ctx.Close()
//
// 	user := ctx.createUser()
// 	client := newTestClient(t).login(user.Id)
//
// 	// Create a course with a module with one unique content, and one shared content
// 	course, modules := sampleCreateCourseInput()
// 	client.createCourse(course, modules)
//
// 	courseId1 := 1
// 	moduleId1 := 1
//
// 	newModuleVersion1 := db.NewModuleVersion(2, moduleId1, 1, "new title", "new description")
// 	contentStr := "qcontent1"
// 	contentStr2 := "qcontent2"
// 	question := newUiQuestionBuilder().
// 		text("qname1").
// 		choice("qchoice1", true).
// 		explain("qexplanation1").
// 		build()
// 	blocks := []blockInput{
// 		newQuestionBlockInput(question),
// 		newContentBlockInput(contentStr),
// 		newContentBlockInput(contentStr2),
// 	}
// 	client.editModule(int64(courseId1), newModuleVersion1, blocks)
//
// 	// Create a course with a module with one shared content
// 	client.createCourse(course, modules)
//
// 	courseId2 := 2
// 	moduleId2 := 2
//
// 	newModuleVersion2 := db.NewModuleVersion(2, moduleId2, 1, "new title", "new description")
// 	blocks = []blockInput{
// 		newContentBlockInput(contentStr),
// 	}
// 	client.editModule(int64(courseId2), newModuleVersion2, blocks)
//
// 	// Delete first courses module
// 	client.deleteModule(courseId1, moduleId1)
//
// 	// require shared content stays
// 	// require unique content is deleted
// 	content, err := ctx.db.GetAllContent()
// 	require.Nil(t, err)
// 	require.Len(t, content, 1)
// 	require.Contains(t, content[0].Content, contentStr)
// }

func TestStudentCoursePage(t *testing.T) {
	ctx := startServer(t)
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestNoobClient(user.Id)

	course, module := createSampleCourse(t, client, 1, 1)
	populateSampleModule(t, client, noob_client.ModuleUpdate{Id: 1, Title: module.Title, Description: module.Description}, 1, 1)

	resp := client.EnrollCourse(int64(course.Id))
	require.Equal(t, 200, resp.StatusCode)

	body := getPageBody(t, client, studentCoursePageRoute(course.Id))
	require.Contains(t, body, course.Title)
	require.Equal(t, strings.Count(body, module.Title), 1)

	// If we enroll again in the same course it should not succeed
	resp = client.EnrollCourse(int64(course.Id))
	require.NotEqual(t, 200, resp.StatusCode)
}

// Test module version, i.e. once someone has visited the module, then when you edit it
// and they go back, it's still there
// func TestModuleVersioning(t *testing.T) {
// 	ctx := startServer(t)
// 	defer ctx.Close()
//
// 	user := ctx.createUser()
// 	client := newTestClient(t).login(user.Id)
//
// 	// Create module with content
// 	course, modules := sampleCreateCourseInput()
// 	client.createCourse(course, modules)
//
// 	courseId := 1
// 	moduleId := 1
//
// 	// Create one version
// 	newModuleVersion1 := db.NewModuleVersion(2, moduleId, 1, "new title", "new description")
// 	contentStr := "qcontent1"
// 	question := newUiQuestionBuilder().
// 			text("qname1").
// 			choice("qchoice1", false).
// 			choice("qchoice2", true).
// 			explain("qexplanation1").
// 			build()
// 	blocks := []blockInput{
// 		newContentBlockInput(contentStr),
// 		newQuestionBlockInput(question),
// 	}
// 	client.editModule(int64(courseId), newModuleVersion1, blocks)
//
// 	// Visit
// 	client.enrollCourse(courseId)
// 	body := client.getPageBody(studentCoursePageRoute(courseId))
// 	require.Contains(t, body, takeModulePageRoute(courseId, moduleId))
//
// 	// Take module initial page (first block = content)
// 	body = client.getPageBody(takeModulePageRoute(courseId, moduleId))
// 	require.Contains(t, body, contentStr)
// 	require.NotContains(t, body, question.Content.Content)
// 	require.Contains(t, body, takeModulePieceRoute(courseId, moduleId, 1))
//
// 	// Next piece (question)
// 	body = client.getPageBody(takeModulePieceRoute(courseId, moduleId, 1))
// 	require.Contains(t, body, question.Content.Content)
// 	require.Contains(t, body, question.Choices[0].Content.Content)
// 	require.Contains(t, body, question.Choices[1].Content.Content)
// 	require.NotContains(t, body, question.Explanation.Content)
//
// 	// TODO: Answer question + reveal explanation
//
// 	// Edit
// 	newModuleVersion2 := db.NewModuleVersion(3, moduleId, 2, "new title2", "new description2")
// 	contentStr2 := "qcontent2"
// 	question2 := newUiQuestionBuilder().
// 			text("q2name1").
// 			choice("q2choice1", false).
// 			choice("q2choice2", true).
// 			explain("q2explanation1").
// 			build()
// 	blocks2 := []blockInput{
// 		newQuestionBlockInput(question2),
// 		newContentBlockInput(contentStr2),
// 	}
// 	client.editModule(int64(courseId), newModuleVersion2, blocks2)
//
// 	// Visit again, and show old version
// 	body = client.getPageBody(takeModulePageRoute(courseId, moduleId))
// 	require.Contains(t, body, contentStr)
// 	require.Contains(t, body, question.Content.Content)
// 	require.Contains(t, body, question.Choices[0].Content.Content)
// 	require.Contains(t, body, question.Choices[1].Content.Content)
// 	require.NotContains(t, body, question.Explanation.Content)
// }

func TestPrerequisites(t *testing.T) {
	ctx := startServer(t)
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	moduleInputs := []titleDescInput{
		newTitleDescInput("module1", "desc1"),
		newTitleDescInput("module2", "desc2"),
		newTitleDescInput("module3", "desc3"),
		newTitleDescInput("module4", "desc4"),
	}

	client.createCourse(newTitleDescInput("course", "description"), moduleInputs)

	// Make all modules visible by adding a content block to them
	courseId := 1
	for i := 0; i < len(moduleInputs); i++ {
		moduleId := i + 1
		newModuleVersion := db.NewModuleVersion(-1, moduleId, -1, moduleInputs[i].Title, moduleInputs[i].Description)
		client.editModule(int64(courseId), newModuleVersion, []blockInput{
			newContentBlockInput(moduleInputs[i].Title + "content"),
			newContentBlockInput(moduleInputs[i].Title + "content2"),
		})
	}

	// Assert with no prereqs they all just show up on the course page
	client.enrollCourse(courseId)
	body := client.getPageBody(studentCoursePageRoute(courseId))
	for _, moduleInput := range moduleInputs {
		require.Contains(t, body, moduleInput.Title)
	}

	// Set a prereq graph of a diamond and assert you must take
	// them in sequence. I.e. prev module id is a prereq.
	client.setPrereqs(courseId, 2, []int{1})
	client.setPrereqs(courseId, 3, []int{1})
	client.setPrereqs(courseId, 4, []int{2, 3})

	// Test that we cannot make prereq cycles
	client.setPrereqsFail(courseId, 1, []int{1})
	client.setPrereqsFail(courseId, 1, []int{2})
	client.setPrereqsFail(courseId, 1, []int{3})
	client.setPrereqsFail(courseId, 1, []int{4})
	client.setPrereqsFail(courseId, 2, []int{4})
	client.setPrereqsFail(courseId, 3, []int{4})

	// Only module 1 should show up
	body = client.getPageBody(studentCoursePageRoute(courseId))
	require.Contains(t, body, moduleInputs[0].Title)
	require.NotContains(t, body, moduleInputs[1].Title)
	require.NotContains(t, body, moduleInputs[2].Title)
	require.NotContains(t, body, moduleInputs[3].Title)

	// Test that we cannot just take subsequent modules without prereqs
	client.getPageFail(takeModulePageRoute(courseId, 2))
	client.getPageFail(nextModulePieceRoute(courseId, 2, 1))
	client.getPageFail(takeModulePageRoute(courseId, 3))
	client.getPageFail(nextModulePieceRoute(courseId, 3, 1))
	client.getPageFail(takeModulePageRoute(courseId, 4))
	client.getPageFail(nextModulePieceRoute(courseId, 4, 1))

	takeModule := func(moduleId int) {
		body := client.getPageBody(takeModulePageRoute(courseId, moduleId))
		require.Contains(t, body, nextModulePieceRoute(courseId, moduleId, 1))
		body = client.getPageBody(nextModulePieceRoute(courseId, moduleId, 1))
		require.Contains(t, body, completeModuleRoute(courseId, moduleId))
		client.completeModule(courseId, moduleId)
	}

	takeModule(1)

	// Module 2 and 3 should be unlocked
	body = client.getPageBody(studentCoursePageRoute(courseId))
	require.Contains(t, body, moduleInputs[0].Title) // Completed
	require.Contains(t, body, moduleInputs[1].Title)
	require.Contains(t, body, moduleInputs[2].Title)
	require.NotContains(t, body, moduleInputs[3].Title)

	takeModule(2)

	// Module 4 should still be locked
	body = client.getPageBody(studentCoursePageRoute(courseId))
	require.NotContains(t, body, moduleInputs[3].Title)

	takeModule(3)
	
	// Module 4 should now be unlocked
	body = client.getPageBody(studentCoursePageRoute(courseId))
	require.Contains(t, body, moduleInputs[3].Title)
}

func TestPoints(t *testing.T) {
	ctx := startServer(t)
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestClient(t).login(user.Id)

	moduleInputs := []titleDescInput{
		newTitleDescInput("module1", "desc1"),
		newTitleDescInput("module2", "desc2"),
	}
	client.createCourse(newTitleDescInput("course", "description"), moduleInputs)

	courseId := 1
	for i := 0; i < len(moduleInputs); i++ {
		moduleId := i + 1
		newModuleVersion := db.NewModuleVersion(-1, moduleId, -1, moduleInputs[i].Title, moduleInputs[i].Description)
		client.editModule(int64(courseId), newModuleVersion, []blockInput{newContentBlockInput(moduleInputs[i].Title + "content")})
	}

	client.enrollCourse(courseId)

	getTotalPoints := func() int {
		points := 0
		for _, moduleId := range []int{1, 2} {
			point, err := ctx.db.GetPoint(user.Id, moduleId)
			if err != nil && err != sql.ErrNoRows {
				t.Fatal(err)
			}
			points += point.Count
		}
		return points
	}

	require.Equal(t, 0, getTotalPoints())

	// Take module 1
	body := client.getPageBody(takeModulePageRoute(courseId, 1))
	require.Contains(t, body, completeModuleRoute(courseId, 1))
	client.completeModule(courseId, 1)

	require.Equal(t, 1, getTotalPoints())

	// Take module 2
	body = client.getPageBody(takeModulePageRoute(courseId, 2))
	require.Contains(t, body, completeModuleRoute(courseId, 2))
	client.completeModule(courseId, 2)

	require.Equal(t, 2, getTotalPoints())

	// Mark complete again, and shouldn't get more points
	client.completeModule(courseId, 1)
	require.Equal(t, 2, getTotalPoints())
	client.completeModule(courseId, 2)
	require.Equal(t, 2, getTotalPoints())
}

const testModule = `
---
title: m1
description: asdf
---


[//]: # (question)
hello? $x=3_2$asdf

[//]: # (choice correct)
wow! $x=5_3$

[//]: # (choice)
nice!

[//]: # (explanation)
# what is up?

how is it going?

[//]: # (question)
testq

[//]: # (choice)
testc 1

[//]: # (choice correct)
testc 2

[//]: # (content)
## woohoo!

yea!
`

func parseModule(module string) (titleDescInput, []blockInput, error) {
	metadataUnseen := 0
	metadataProcessing := 1
	metadataParsed := 2
	metadataStatus := metadataUnseen
	moduleTitle := ""
	moduleDescription := ""
	parsingNothing := 0
	parsingContent := 1
	parsingQuestion := 2
	parsingChoice := 3
	parsingCorrectChoice := 4
	parsingExplanation := 5
	parsingType := parsingNothing
	buffer := []string{}
	blockInputs := []blockInput{}
	questionBuilder := newUiQuestionBuilder()

	finishPiece := func(parsingType int, newParsingType int, buffer []string, questionBuilder *uiQuestionBuilder, blockInputs *[]blockInput) error {
		text := strings.Join(buffer, "\n")
		text = strings.TrimSpace(text)
		if parsingType == parsingContent {
			*blockInputs = append(*blockInputs, newContentBlockInput(text))
		} else if parsingType == parsingQuestion {
			*questionBuilder = questionBuilder.text(text)
		} else if parsingType == parsingChoice {
			*questionBuilder = questionBuilder.choice(text, false)
		} else if parsingType == parsingCorrectChoice {
			*questionBuilder = questionBuilder.choice(text, true)
		} else if parsingType == parsingExplanation {
			*questionBuilder = questionBuilder.explain(text)
		}

		if parsingType == parsingQuestion && !(newParsingType == parsingChoice || newParsingType == parsingCorrectChoice) {
			return fmt.Errorf("question must be followed by choice or correct choice")
		}

		justParsedChoice := parsingType == parsingChoice || parsingType == parsingCorrectChoice
		nextParsingNonQuestion := newParsingType != parsingChoice && newParsingType != parsingCorrectChoice && newParsingType != parsingExplanation
		justParsedExplanation := parsingType == parsingExplanation
		finishedQuestion := justParsedExplanation || (justParsedChoice && nextParsingNonQuestion)
		if finishedQuestion {
			*blockInputs = append(*blockInputs, newQuestionBlockInput(questionBuilder.build()))
			*questionBuilder = newUiQuestionBuilder()
		}
		return nil
	}

	scanner := bufio.NewScanner(strings.NewReader(module))

	for scanner.Scan() {
		line := scanner.Text()
		if metadataStatus == metadataUnseen && line == "" {
			continue
		}
		if metadataStatus == metadataUnseen && line == "---" {
			metadataStatus = metadataProcessing
			continue
		}
		if metadataStatus == metadataProcessing && line == "---" {
			metadataStatus = metadataParsed
			continue
		}
		if metadataStatus == metadataProcessing {
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) != 2 {
				return titleDescInput{}, nil, fmt.Errorf("metadata not key value")
			}
			key := parts[0]
			value := parts[1]
			if key == "title" {
				moduleTitle = value
			} else if key == "description" {
				moduleDescription = value
			}
			continue
		}
		if metadataStatus != metadataParsed {
			return titleDescInput{}, nil, fmt.Errorf("metadata not parsed")
		}

		pattern := `^\[//\]: # \((.+?)\)$`
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(line)
		if matches == nil {
			buffer = append(buffer, line)
			continue
		}

		// The first element is the whole match, the second is the captured group
		parsedValue := matches[1]
		values := strings.Split(parsedValue, " ")
		valueType := values[0]
		newParsingType := parsingNothing
		switch valueType {
		case "content":
			newParsingType = parsingContent
		case "question":
			newParsingType = parsingQuestion
		case "choice":
			newParsingType = parsingChoice
			if len(values) == 2 && values[1] == "correct" {
				newParsingType = parsingCorrectChoice
			}
		case "explanation":
			newParsingType = parsingExplanation
		}

		// If we matched a new block, it means we're at the end
		// of the previous block
		finishPiece(parsingType, newParsingType, buffer, &questionBuilder, &blockInputs)

		buffer = []string{}
		parsingType = newParsingType
	}

	finishPiece(parsingType, parsingNothing, buffer, &questionBuilder, &blockInputs)

	if err := scanner.Err(); err != nil {
		return titleDescInput{}, nil, err
	}
	if metadataStatus != metadataParsed {
		return titleDescInput{}, nil, fmt.Errorf("metadata not parsed")
	}

	return titleDescInput{moduleTitle, moduleDescription}, blockInputs, nil
}

// func TestFormat(t *testing.T) {
// 	ctx := startServer(t)
// 	defer ctx.Close()
//
// 	user := ctx.createUser()
// 	client := newTestNoobClient(user.Id)
//
// 	modules := []noob_client.ModuleInit{
// 		{Title: "module1", Description: "desc1"},
// 		{Title: "module2", Description: "desc2"},
// 	}
// 	resp := client.CreateCourse("course", "description", true, modules)
// 	require.Equal(t, 200, resp.StatusCode)
//
// 	courseId := int64(1)
// 	for i := 0; i < len(modules); i++ {
// 		moduleId := i + 1
// 		resp = client.EditModule(courseId, int64(moduleId), modules[i].Title, modules[i].Description, []noob_client.Block{
// 			noob_client.NewContentBlock(modules[i].Title + "content"),
// 		})
// 	}
//
// 	resp = client.EnrollCourse(courseId)
// 	require.Equal(t, 200, resp.StatusCode)
//
// 	title, description, blocks, err := noob_client.ParseModule(testModule)
// 	require.Nil(t, err)
//
// 	moduleId := int64(1)
// 	resp = client.EditModule(courseId, moduleId, title, description, blocks)
// 	require.Equal(t, 200, resp.StatusCode)
//
// 	body := getPageBody(t, client, noob_client.ExportModuleRoute(courseId, moduleId))
// 	require.Contains(t, body, title)
// 	require.Contains(t, body, description)
// 	for _, block := range blocks {
// 		switch block.BlockType {
// 		case noob_client.KnowledgePointBlockType:
// 			// TODO: check kp when we change the format
// 		case noob_client.ContentBlockType:
// 			require.Contains(t, body, block.Content.Text)
// 		}
// 	}
//
// 	body = getPageBody(t, client, noob_client.ExportModuleRoute(courseId, moduleId))
// 	require.Equal(t, strings.TrimSpace(testModule), strings.TrimSpace(body))
// }

func getPageBody(t *testing.T, client noob_client.Client, path string) string {
	resp := client.GetPage(path)
	require.Equal(t, 200, resp.StatusCode)
	return bodyText(t, resp)
}

func TestKnowledgePoint(t *testing.T) {
	ctx := startServer(t)
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestNoobClient(user.Id)

	client.CreateCourse("course", "description", true, []noob_client.ModuleInit{})
	courseId := int64(1)

	question1 := noob_client.QuestionBlock {
		Text: "kp1 question",
		Choices: []noob_client.Choice{
			{Text: "kp1 choice1", Correct: false},
			{Text: "kp1 choice2", Correct: true},
		},
		Explanation: "kp1 explanation",
	}
	resp := client.CreateKnowledgePoint(courseId, "kp1", []noob_client.QuestionBlock{question1})
	require.Equal(t, 200, resp.StatusCode)
	kpId1 := 1

	question2 := noob_client.QuestionBlock {
		Text: "kp2 question",
		Choices: []noob_client.Choice{
			{Text: "kp2 choice1", Correct: false},
			{Text: "kp2 choice2", Correct: true},
		},
		Explanation: "kp2 explanation",
	}
	resp = client.CreateKnowledgePoint(courseId, "kp2", []noob_client.QuestionBlock{question2})
	require.Equal(t, 200, resp.StatusCode)
	kpId2 := 2

	checkKnowledgePoint := func(kpId int64, name string, question noob_client.QuestionBlock) {
		body := getPageBody(t, client, noob_client.EditKnowledgePointRoute(courseId, kpId))
		require.Contains(t, body, name)
		require.Contains(t, body, question.Text)
		for _, choice := range question.Choices {
			require.Contains(t, body, choice.Text)
			// TODO: check correct
		}
		require.Contains(t, body, question.Explanation)
	}
		
	checkKnowledgePoint(int64(kpId1), "kp1", question1)
	checkKnowledgePoint(int64(kpId2), "kp2", question2)
}

func TestInputValidationKnowledgePoint(t *testing.T) {
	ctx := startServer(t)
	defer ctx.Close()

	user := ctx.createUser()
	client := newTestNoobClient(user.Id)

	client.CreateCourse("course", "description", true, []noob_client.ModuleInit{})
	courseId := int64(1)

	var resp *http.Response
	kpName := "kp1"
	resp = client.CreateKnowledgePoint(courseId, kpName, []noob_client.QuestionBlock{
		noob_client.NewQuestion("", []noob_client.Choice{
			{Text: "choice1", Correct: true},
		}, ""),
	})
	require.NotEqual(t, 200, resp.StatusCode)

	resp = client.CreateKnowledgePoint(courseId, kpName, []noob_client.QuestionBlock{
		noob_client.NewQuestion(strings.Repeat("a", internal.MaxQuestionLength + 1), []noob_client.Choice{
			{Text: "choice1", Correct: true},
		}, ""),
	})
	require.NotEqual(t, 200, resp.StatusCode)

	resp = client.CreateKnowledgePoint(courseId, kpName, []noob_client.QuestionBlock{
		noob_client.NewQuestion("question", []noob_client.Choice{}, ""),
	})
	require.NotEqual(t, 200, resp.StatusCode)

	resp = client.CreateKnowledgePoint(courseId, kpName, []noob_client.QuestionBlock{
		noob_client.NewQuestion("question", []noob_client.Choice{
			{Text: "", Correct: true},
		}, ""),
	})
	require.NotEqual(t, 200, resp.StatusCode)

	resp = client.CreateKnowledgePoint(courseId, kpName, []noob_client.QuestionBlock{
		noob_client.NewQuestion("question", []noob_client.Choice{
			{Text: strings.Repeat("a", internal.MaxChoiceLength + 1), Correct: true},
		}, ""),
	})
	require.NotEqual(t, 200, resp.StatusCode)

	resp = client.CreateKnowledgePoint(courseId, kpName, []noob_client.QuestionBlock{
		noob_client.NewQuestion("question", []noob_client.Choice{
			{Text: "choice1", Correct: false},
		}, ""),
	})
	require.NotEqual(t, 200, resp.StatusCode)

	tooManyChoices := []noob_client.Choice{}
	for i := 0; i < internal.MaxChoices; i++ {
		tooManyChoices = append(tooManyChoices, noob_client.Choice{Text: "choice", Correct: false})
	}
	tooManyChoices = append(tooManyChoices, noob_client.Choice{Text: "choice", Correct: true})
	resp = client.CreateKnowledgePoint(courseId, kpName, []noob_client.QuestionBlock{
		noob_client.NewQuestion("question", tooManyChoices, ""),
	})
	require.NotEqual(t, 200, resp.StatusCode)
}
