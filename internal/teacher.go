package internal

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"noobular/internal/db"
)

// Courses page

func getTeacherUiModulesForCourse(ctx HandlerContext, courseId int) ([]UiModule, error) {
	modules, err := ctx.dbClient.GetModules(courseId)
	if err != nil {
		return []UiModule{}, err
	}
	uiModules := make([]UiModule, 0)
	for _, module := range modules {
		moduleVersion, err := ctx.dbClient.GetLatestModuleVersion(module.Id)
		if err != nil {
			return []UiModule{}, err
		}
		uiModules = append(uiModules, NewUiModuleTeacher(courseId, moduleVersion))
	}
	return uiModules, nil
}

func handleTeacherCoursesPage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	newCourseId, err := strconv.Atoi(r.URL.Query().Get("newCourse"))
	if err != nil {
		newCourseId = -1
	}
	courses, err := ctx.dbClient.GetTeacherCourses(user.Id)
	if err != nil {
		return err
	}
	uiCourses := make([]UiCourse, len(courses))
	for i, course := range courses {
		uiModules, err := getTeacherUiModulesForCourse(ctx, course.Id)
		if err != nil {
			return err
		}
		uiCourses[i] = NewUiCourse(course, uiModules)
	}
	return ctx.renderer.RenderTeacherCoursePage(w, uiCourses, newCourseId)
}

// Delete course (on teacher courses page)

func handleDeleteCourse(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	err = ctx.dbClient.DeleteCourse(user.Id, courseId)
	if err != nil {
		return err
	}
	return nil
}

// Create course page

func handleCreateCoursePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	return ctx.renderer.RenderCreateCoursePage(w)
}

type createCourseRequest struct {
	title              string
	description        string
	public             bool
	moduleTitles       []string
	moduleDescriptions []string
}

func parseCreateCourseRequest(r *http.Request) (createCourseRequest, error) {
	err := r.ParseForm()
	if err != nil {
		return createCourseRequest{}, err
	}
	title := r.Form.Get("title")
	if title == "" {
		return createCourseRequest{}, fmt.Errorf("Title cannot be empty")
	}
	description := r.Form.Get("description")
	if description == "" {
		return createCourseRequest{}, fmt.Errorf("Description cannot be empty")
	}
	public := r.Form.Get("public") == "on"
	moduleTitles := r.Form["module-title[]"]
	moduleDescriptions := r.Form["module-description[]"]
	if len(moduleDescriptions) != len(moduleTitles) {
		return createCourseRequest{}, fmt.Errorf("Each module must have a description")
	}
	return createCourseRequest{title, description, public, moduleTitles, moduleDescriptions}, nil
}

func handleCreateCourse(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	req, err := parseCreateCourseRequest(r)
	if err != nil {
		return err
	}
	course, err := ctx.dbClient.CreateCourse(user.Id, req.title, req.description, req.public)
	if err != nil {
		return err
	}
	for i := 0; i < len(req.moduleTitles); i++ {
		moduleTitle := req.moduleTitles[i]
		moduleDescription := req.moduleDescriptions[i]
		_, err := ctx.dbClient.CreateModule(course.Id, moduleTitle, moduleDescription)
		if err != nil {
			return err
		}
	}
	w.Header().Add("HX-Redirect", fmt.Sprintf("/teacher?newCourse=%d#course-%d", course.Id, course.Id))
	return nil
}

// Edit course page

func handleEditCoursePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	course, err := ctx.dbClient.GetEditCourse(user.Id, courseId)
	if err != nil {
		return err
	}
	uiModules, err := getTeacherUiModulesForCourse(ctx, course.Id)
	if err != nil {
		return err
	}
	enrollmentCount, err := ctx.dbClient.GetEnrollmentCount(courseId)
	if err != nil {
		return err
	}
	uiCourse := NewUiCourse(course, uiModules)
	publicFixed := enrollmentCount > 0
	return ctx.renderer.RenderEditCoursePage(w, uiCourse, publicFixed)
}

type editCourseRequest struct {
	courseId           int
	title              string
	description        string
	public             bool
	moduleIds          []int
	moduleTitles       []string
	moduleDescriptions []string
}

func parseEditCourseRequest(r *http.Request) (editCourseRequest, error) {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return editCourseRequest{}, err
	}
	err = r.ParseForm()
	if err != nil {
		return editCourseRequest{}, err
	}
	title := r.Form.Get("title")
	description := r.Form.Get("description")
	public := r.Form.Get("public") == "on"
	moduleIdStrs := r.Form["module-id[]"]
	moduleIds := make([]int, len(moduleIdStrs))
	for i, moduleIdStr := range moduleIdStrs {
		moduleIdInt, err := strconv.Atoi(moduleIdStr)
		if err != nil {
			return editCourseRequest{}, err
		}
		moduleIds[i] = moduleIdInt
	}
	moduleTitles := r.Form["module-title[]"]
	moduleDescriptions := r.Form["module-description[]"]
	moduleIdCount := len(moduleIds)
	moduleTitleCount := len(moduleTitles)
	moduleDescriptionCount := len(moduleDescriptions)
	if moduleIdCount != moduleTitleCount || moduleTitleCount != moduleDescriptionCount {
		return editCourseRequest{}, fmt.Errorf("Edit course module data lengths are misaligned: %d module ids, %d moduleTitles, %d motule descriptions", moduleIdCount, moduleTitleCount, moduleDescriptionCount)
	}
	return editCourseRequest{courseId, title, description, public, moduleIds, moduleTitles, moduleDescriptions}, nil
}

func handleEditCourse(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	req, err := parseEditCourseRequest(r)
	if err != nil {
		return err
	}
	_, err = ctx.dbClient.GetTeacherCourse(req.courseId, user.Id)
	if err != nil {
		return err
	}
	tx, err := ctx.dbClient.Begin()
	defer tx.Rollback()
	if err != nil {
		return err
	}
	enrollmentCount, err := db.GetEnrollmentCount(tx, req.courseId)
	if err != nil {
		return err
	}
	if enrollmentCount > 0 {
		req.public = true
	}
	course, err := db.EditCourse(tx, user.Id, req.courseId, req.title, req.description, req.public)
	if err != nil {
		return err
	}
	modules := make([]db.Module, len(req.moduleTitles))
	for i := 0; i < len(req.moduleTitles); i++ {
		moduleId := req.moduleIds[i]
		moduleTitle := req.moduleTitles[i]
		moduleDescription := req.moduleDescriptions[i]
		// -1 means this is a new module
		if moduleId == -1 {
			module, err := db.CreateModule(tx, req.courseId, moduleTitle, moduleDescription)
			if err != nil {
				return err
			}
			moduleId = module.Id
		} else {
			_, err = db.GetModule(tx, req.courseId, moduleId)
			if err != nil {
				return err
			}
			// No need to instert new module version just to change the name.
			version, err := db.GetLatestModuleVersion(tx, moduleId)
			if err != nil {
				return err
			}
			err = db.UpdateModuleVersionMetadata(tx, version.Id, moduleTitle, moduleDescription)
			if err != nil {
				return err
			}
		}
		module := db.NewModule(moduleId, course.Id)
		modules[i] = module
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return ctx.renderer.RenderCourseEdited(w)
}

// Add element generic template
// These are used in multiple pages, for adding modules, questions, choices.

// These two handlers seem kinda dumb, i.e. they could just be done in javascript,
// but I'm just going to do things the pure HTMX way for now to see how it goes.

// Simply returns another small chunk of HTML to add new elements
func handleAddElement(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	element := r.PathValue("element")
	var err error
	if element == "module" {
		err = ctx.renderer.RenderNewModule(w, EmptyModule())
	} else if element == "question" {
		err = ctx.renderer.RenderNewQuestion(w, EmptyQuestion())
	} else if element == "content" {
		err = ctx.renderer.RenderNewContent(w, EmptyContent())
	} else {
		err = fmt.Errorf("Unknown element: %s", element)
	}
	return err
}

func handleAddChoice(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	questionIdx, err := strconv.Atoi(r.PathValue("questionIdx"))
	if err != nil {
		return err
	}
	return ctx.renderer.RenderNewChoice(w, EmptyChoice(questionIdx))
}

func handleDeleteElement(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	// No op
	return nil
}

func handleDeleteModule(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return err
	}
	_, err = ctx.dbClient.GetModuleCourse(user.Id, moduleId)
	if err != nil {
		return err
	}
	err = ctx.dbClient.DeleteModule(moduleId)
	if err != nil {
		return err
	}
	// Nothing to render
	return nil
}

// Edit module page

func handleEditModulePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return err
	}
	course, err := ctx.dbClient.GetEditCourse(user.Id, courseId)
	if err != nil {
		return err
	}
	moduleVersion, err := ctx.dbClient.GetLatestModuleVersion(moduleId)
	if err != nil {
		return fmt.Errorf("Error getting module version: %w", err)
	}
	blocks, err := ctx.dbClient.GetBlocks(moduleVersion.Id)
	if err != nil {
		return fmt.Errorf("Error getting blocks: %w", err)
	}
	uiBlocks := make([]UiBlock, len(blocks))
	for _, block := range blocks {
		uiBlock := UiBlock{BlockType: block.BlockType}
		if block.BlockType == db.ContentBlockType {
			content, err := ctx.dbClient.GetContentFromBlock(block.Id)
			if err != nil {
				return err
			}
			uiBlock.Content = NewUiContent(content)
		} else if block.BlockType == db.QuestionBlockType {
			question, err := ctx.dbClient.GetQuestionFromBlock(block.Id)
			if err != nil {
				return err
			}
			choices, err := ctx.dbClient.GetChoicesForQuestion(question.Id)
			if err != nil {
				return err
			}
			explanation, err := ctx.dbClient.GetExplanationForQuestion(question.Id)
			if err != nil {
				return err
			}
			uiBlock.Question = NewUiQuestionEdit(question, choices, explanation)
		} else {
			return fmt.Errorf("invalid block type: %s", block.BlockType)
		}
		uiBlocks = append(uiBlocks, uiBlock)
	}
	return ctx.renderer.RenderEditModulePage(w, UiEditModule{
		CourseId:    courseId,
		CourseTitle: course.Title,
		ModuleId:    moduleId,
		ModuleTitle: moduleVersion.Title,
		ModuleDesc:  moduleVersion.Description,
		Blocks:      uiBlocks,
	})
}

type editModuleRequest struct {
	moduleId          int
	title             string
	description       string
	blockTypes        []string
	contents          []string
	questions         []string
	choicesByQuestion [][]string
	correctChoiceIdxs []int
	explanations      []string
}

func parseEditModuleRequest(r *http.Request) (editModuleRequest, error) {
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return editModuleRequest{}, err
	}
	err = r.ParseForm()
	if err != nil {
		return editModuleRequest{}, err
	}
	log.Println("Form:", r.Form)
	title := r.Form.Get("title")
	description := r.Form.Get("description")
	blockTypes := r.Form["block-type[]"]
	contents := r.Form["content-text[]"]
	questions := r.Form["question-title[]"]
	questionIdxs := r.Form["question-idx[]"]
	if len(questions) != len(questionIdxs) {
		return editModuleRequest{}, fmt.Errorf("Each question must have an index")
	}
	explanations := r.Form["question-explanation[]"]
	choices := r.Form["choice-title[]"]
	// These match choices 1-1 (including having "end-choice")
	// They are a random number generated to be roughly unique for this choice.
	choiceUiIdxs := r.Form["choice-idx[]"]
	// Choices are separated by "end-choice" in the form
	// i.e. we expect r.Form["choice-title[]"] to look something like:
	// ["choice1", "choice2", "end-choice", "choice3", "choice4", "end-choice"]
	uiQuestions := make([]string, len(questions))
	uiChoicesByQuestion := make([][]string, len(questions))
	correctChoicesByQuestion := make([]int, len(questions))
	choiceIdx := 0
	for i, question := range questions {
		uiChoices := make([]string, 0)
		// This holds the choiceUiIdx of the correct choice for each question.
		correctChoiceIdx := r.Form.Get(fmt.Sprintf("correct-choice-%s", questionIdxs[i]))
		if correctChoiceIdx == "" {
			return editModuleRequest{}, fmt.Errorf("Each question must have a correct choice")
		}
		for ; choiceIdx < len(choices); choiceIdx++ {
			choice := choices[choiceIdx]
			if choice == "end-choice" {
				choiceIdx++
				break
			}
			uiChoices = append(uiChoices, choice)
			if choiceUiIdxs[choiceIdx] == correctChoiceIdx {
				correctChoicesByQuestion[i] = len(uiChoices) - 1
			}
		}
		uiQuestions[i] = question
		uiChoicesByQuestion[i] = uiChoices
	}
	for i, question := range uiQuestions {
		if question == "" {
			return editModuleRequest{}, fmt.Errorf("Questions cannot be empty")
		}
		if len(uiChoicesByQuestion[i]) == 0 {
			return editModuleRequest{}, fmt.Errorf("Questions must have at least one choice")
		}
		for _, choice := range uiChoicesByQuestion[i] {
			if choice == "" {
				return editModuleRequest{}, fmt.Errorf("Choices cannot be empty")
			}
		}
	}
	for _, content := range contents {
		if content == "" {
			return editModuleRequest{}, fmt.Errorf("Contents cannot be empty")
		}
	}
	// TODO: maybe remove this. This is just so I can restrict what I have to consider
	// rendering in the UI for now.
	if len(uiQuestions) > 12 {
		return editModuleRequest{}, fmt.Errorf("Modules cannot have more than 12 questions")
	}
	return editModuleRequest{
		moduleId,
		title,
		description,
		blockTypes,
		contents,
		uiQuestions,
		uiChoicesByQuestion,
		correctChoicesByQuestion,
		explanations,
	}, nil
}

func handleEditModule(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	req, err := parseEditModuleRequest(r)
	if err != nil {
		return err
	}
	_, err = ctx.dbClient.GetModuleCourse(user.Id, req.moduleId)
	if err != nil {
		return fmt.Errorf("Module %d not found", req.moduleId)
	}
	tx, err := ctx.dbClient.Begin()
	defer tx.Rollback()
	if err != nil {
		return err
	}
	version, err := db.InsertModuleVersion(tx, req.moduleId, req.title, req.description)
	if err != nil {
		return err
	}
	questionIdx := 0
	contentIdx := 0
	for i, blockType := range req.blockTypes {
		blockId, err := db.InsertBlock(tx, version.Id, i, db.BlockType(blockType))
		if err != nil {
			return err
		}
		if db.BlockType(blockType) == db.ContentBlockType {
			err = db.InsertContentBlock(tx, blockId, req.contents[contentIdx])
			if err != nil {
				return err
			}
			contentIdx += 1
		} else if db.BlockType(blockType) == db.QuestionBlockType {
			err = db.InsertQuestion(tx, blockId, req.questions[questionIdx], req.choicesByQuestion[questionIdx], req.correctChoiceIdxs[questionIdx], req.explanations[questionIdx])
			if err != nil {
				return err
			}
			questionIdx += 1
		} else {
			return fmt.Errorf("invalid block type: %s", blockType)
		}
	}
	// Delete previous version if no one is pinned to it
	visitCount, err := db.GetVisitCount(tx, req.moduleId, version.VersionNumber-1)
	if err != nil {
		return err
	}
	if visitCount == 0 {
		err = db.DeleteModuleVersion(tx, req.moduleId, version.VersionNumber-1)
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return ctx.renderer.RenderModuleEdited(w)
}

// Preview page

func handlePreviewModulePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	// Check they can access this
	course, err := ctx.dbClient.GetTeacherCourse(courseId, user.Id)
	if err != nil {
		return err
	}
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return err
	}
	module, err := ctx.dbClient.GetModule(courseId, moduleId)
	if err != nil {
		return err
	}
	moduleVersion, err := ctx.dbClient.GetLatestModuleVersion(module.Id)
	if err != nil {
		return err
	}
	blocks, err := ctx.dbClient.GetBlocks(moduleVersion.Id)
	blockCount := len(blocks)
	uiBlocks := make([]UiBlock, blockCount)
	for blockIdx := 0; blockIdx < blockCount; blockIdx++ {
		uiBlock, err := getBlock(ctx, moduleVersion.Id, blockIdx, user.Id)
		if err != nil {
			return fmt.Errorf("Error getting block %d for module %d: %v", blockIdx, moduleId, err)
		}
		uiBlocks[blockIdx] = uiBlock
	}
	uiModule := UiTakeModulePage{
		Module:     NewUiModuleStudent(course.Id, moduleVersion, blockCount, false, time.Now(), 0),
		Blocks:     uiBlocks,
		VisitIndex: blockCount,
		Preview:    true,
	}
	return ctx.renderer.RenderTakeModulePage(w, uiModule)
}

// Prereqs page

func uiPrereqsFromModules(modules []db.Module, uiModuleMap map[int]UiModule, prereqs []db.Prereq) []UiPrereq {
	uiPrereqs := make([]UiPrereq, 0)
	prereqSet := make(map[int]bool)
	for _, prereq := range prereqs {
		prereqSet[prereq.PrereqModuleId] = true
	}
	for _, module := range modules {
		uiPrereqs = append(uiPrereqs, NewUiPrereq(uiModuleMap[module.Id], prereqSet[module.Id]))
	}
	return uiPrereqs
}

func handlePrereqPage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	course, err := ctx.dbClient.GetTeacherCourse(courseId, user.Id)
	if err != nil {
		return err
	}
	modules, err := ctx.dbClient.GetModules(course.Id)
	if err != nil {
		return err
	}
	uiModules, err := getTeacherUiModulesForCourse(ctx, course.Id)
	if err != nil {
		return err
	}
	uiModuleMap := make(map[int]UiModule)
	for _, uiModule := range uiModules {
		uiModuleMap[uiModule.Id] = uiModule
	}
	var uiModule UiModule
	uiPrereqs := make([]UiPrereq, 0)
	if len(modules) > 0 {
		uiModule = uiModuleMap[modules[0].Id]
		prereqs, err := ctx.dbClient.GetPrereqs(modules[0].Id)
		if err != nil {
			return err
		}
		uiPrereqs = uiPrereqsFromModules(modules, uiModuleMap, prereqs)
	}
	return ctx.renderer.RenderPrereqPage(w, UiPrereqPageArgs{NewUiCourse(course, uiModules), UiPrereqForm{uiModule, uiPrereqs}})
}

func handlePrereqForm(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	course, err := ctx.dbClient.GetTeacherCourse(courseId, user.Id)
	if err != nil {
		return err
	}
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return err
	}
	modules, err := ctx.dbClient.GetModules(course.Id)
	if err != nil {
		return err
	}
	uiModules, err := getTeacherUiModulesForCourse(ctx, course.Id)
	if err != nil {
		return err
	}
	uiModuleMap := make(map[int]UiModule)
	for _, uiModule := range uiModules {
		uiModuleMap[uiModule.Id] = uiModule
	}
	prereqs, err := ctx.dbClient.GetPrereqs(moduleId)
	if err != nil {
		return err
	}
	uiPrereqs := uiPrereqsFromModules(modules, uiModuleMap, prereqs)
	uiModule, ok := uiModuleMap[moduleId]
	if !ok {
		return fmt.Errorf("Module %d not found", moduleId)
	}
	return ctx.renderer.RenderPrereqForm(w, UiPrereqForm{uiModule, uiPrereqs})
}

type prereqRequest struct {
	courseId        int
	moduleId        int
	prereqModuleIds map[int]bool
}

func parsePrereqRequest(r *http.Request) (prereqRequest, error) {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return prereqRequest{}, err
	}
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return prereqRequest{}, err
	}
	err = r.ParseForm()
	if err != nil {
		return prereqRequest{}, err
	}
	prereqModuleIds := make(map[int]bool)
	for _, prereqModuleIdStr := range r.Form["prereqs[]"] {
		prereqModuleId, err := strconv.Atoi(prereqModuleIdStr)
		if err != nil {
			return prereqRequest{}, err
		}
		prereqModuleIds[prereqModuleId] = true
	}
	return prereqRequest{courseId, moduleId, prereqModuleIds}, nil
}

func hasCycle(edges map[int][]int, curr int, visited map[int]bool) bool {
	if _, ok := visited[curr]; ok {
		return true
	}
	visited[curr] = true
	for _, next := range edges[curr] {
		if hasCycle(edges, next, visited) {
			return true
		}
	}
	delete(visited, curr)
	return false
}

func handleEditPrereqs(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	req, err := parsePrereqRequest(r)
	if err != nil {
		return err
	}
	_, err = ctx.dbClient.GetTeacherCourse(req.courseId, user.Id)
	if err != nil {
		return err
	}
	moduleVersion, err := ctx.dbClient.GetLatestModuleVersion(req.moduleId)
	if err != nil {
		return err
	}
	modules, err := ctx.dbClient.GetModules(req.courseId)
	if err != nil {
		return err
	}

	// Detect cycles using other existing prereqs
	edges := make(map[int][]int) // prereqid -> modules
	for prereqModuleId := range req.prereqModuleIds {
		edges[prereqModuleId] = append(edges[prereqModuleId], req.moduleId)
	}
	for _, module := range modules {
		if module.Id == req.moduleId {
			continue
		}
		prereqs, err := ctx.dbClient.GetPrereqs(module.Id)
		if err != nil {
			return err
		}
		for _, prereq := range prereqs {
			edges[prereq.PrereqModuleId] = append(edges[prereq.PrereqModuleId], module.Id)
		}
	}
	if hasCycle(edges, req.moduleId, make(map[int]bool)) {
		return fmt.Errorf("Cannot create cycle in prereqs")
	}

	tx, err := ctx.dbClient.Begin()
	defer tx.Rollback()
	if err != nil {
		return err
	}
	for _, prereqModule := range modules {
		if _, ok := req.prereqModuleIds[prereqModule.Id]; ok {
			_, err = db.InsertPrereq(tx, req.moduleId, prereqModule.Id)
		} else {
			err = db.DeletePrereq(tx, req.moduleId, prereqModule.Id)
		}
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return ctx.renderer.RenderPrereqEditedResponse(w, NewUiModuleTeacher(req.courseId, moduleVersion))
}
