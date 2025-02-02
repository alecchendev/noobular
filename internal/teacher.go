package internal

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
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
	description := r.Form.Get("description")
	public := r.Form.Get("public") == "on"
	moduleTitles := r.Form["module-title[]"]
	moduleDescriptions := r.Form["module-description[]"]
	return createCourseRequest{title, description, public, moduleTitles, moduleDescriptions}, nil
}

const TitleMaxLength = 128
const DescriptionMaxLength = 1024
const MaxModules = 128

func validateCourseRequest(title string, description string, moduleTitles []string, moduleDescriptions []string) error {
	if title == "" {
		return fmt.Errorf("Title cannot be empty")
	}
	if len(title) > TitleMaxLength {
		return fmt.Errorf("Title cannot be longer than %d characters", TitleMaxLength)
	}
	if description == "" {
		return fmt.Errorf("Description cannot be empty")
	}
	if len(description) > DescriptionMaxLength {
		return fmt.Errorf("Description cannot be longer than %d characters", DescriptionMaxLength)
	}
	if len(moduleDescriptions) != len(moduleTitles) {
		return fmt.Errorf("Each module must have a title and description")
	}
	if len(moduleTitles) > MaxModules {
		return fmt.Errorf("Cannot have more than %d modules", MaxModules)
	}
	for _, moduleTitle := range moduleTitles {
		if moduleTitle == "" {
			return fmt.Errorf("Module titles cannot be empty")
		}
		if len(moduleTitle) > TitleMaxLength {
			return fmt.Errorf("Module titles cannot be longer than %d characters", TitleMaxLength)
		}
	}
	for _, moduleDescription := range moduleDescriptions {
		if moduleDescription == "" {
			return fmt.Errorf("Module descriptions cannot be empty")
		}
		if len(moduleDescription) > DescriptionMaxLength {
			return fmt.Errorf("Module descriptions cannot be longer than %d characters", DescriptionMaxLength)
		}
	}
	return nil
}

func handleCreateCourse(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	req, err := parseCreateCourseRequest(r)
	if err != nil {
		return fmt.Errorf("Error parsing create course request: %v", err)
	}
	err = validateCourseRequest(req.title, req.description, req.moduleTitles, req.moduleDescriptions)
	if err != nil {
		return fmt.Errorf("Error validating create course request: %v", err)
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
	if moduleIdCount != moduleTitleCount {
		return editCourseRequest{}, fmt.Errorf("Edit course module data lengths are misaligned: %d module ids, %d moduleTitles", moduleIdCount, moduleTitleCount)
	}
	return editCourseRequest{courseId, title, description, public, moduleIds, moduleTitles, moduleDescriptions}, nil
}

func handleEditCourse(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	req, err := parseEditCourseRequest(r)
	if err != nil {
		return fmt.Errorf("Error parsing edit course request: %v", err)
	}
	err = validateCourseRequest(req.title, req.description, req.moduleTitles, req.moduleDescriptions)
	if err != nil {
		return fmt.Errorf("Error validating edit course request: %v", err)
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

// TODO: The set of knowledge points should really only be loaded once
// on the first page load.
func handleAddKnowledgePoint(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	_, err = ctx.dbClient.GetTeacherCourse(courseId, user.Id)
	if err != nil {
		return err
	}
	knowledgePoints, err := ctx.dbClient.GetKnowledgePoints(int64(courseId))
	if err != nil {
		return err
	}
	uiKnowledgePoints := make([]UiKnowledgePointDropdownItem, 0)
	for _, kp := range knowledgePoints {
		uiKnowledgePoints = append(uiKnowledgePoints, NewUiKnowledgePointDropdownItem(kp.Id, kp.Name, false))
	}
	return ctx.renderer.RenderNewKnowledgePoint(w, UiKnowledgePointDropdown{
		KnowledgePoints: uiKnowledgePoints,
	})
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

func getQuestion(ctx HandlerContext, question db.Question) (db.Content, []db.Choice, []db.Content, db.Content, error) {
	questionContent, err := ctx.dbClient.GetContent(question.ContentId)
	if err != nil {
		return db.Content{}, []db.Choice{}, []db.Content{}, db.Content{}, fmt.Errorf("Error getting content for question %d: %w", question.Id, err)
	}
	choices, err := ctx.dbClient.GetChoicesForQuestion(question.Id)
	if err != nil {
		return db.Content{}, nil, nil, db.Content{}, fmt.Errorf("Error getting choices for question %d: %w", question.Id, err)
	}
	choiceContents := make([]db.Content, 0)
	for _, choice := range choices {
		choiceContent, err := ctx.dbClient.GetContent(choice.ContentId)
		if err != nil {
			return db.Content{}, nil, nil, db.Content{}, fmt.Errorf("Error getting content for choice %d: %w", choice.Id, err)
		}
		choiceContents = append(choiceContents, choiceContent)
	}
	explanation, err := ctx.dbClient.GetExplanationForQuestion(question.Id)
	if err != nil {
		return db.Content{}, nil, nil, db.Content{}, err
	}
	return questionContent, choices, choiceContents, explanation, nil
}

func handleEditModulePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return err
	}
	course, err := ctx.dbClient.GetTeacherCourse(courseId, user.Id)
	if err != nil {
		return err
	}
	knowledgePoints, err := ctx.dbClient.GetKnowledgePoints(int64(courseId))
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
	uiBlocks := make([]UiEditModuleBlock, len(blocks))
	for _, block := range blocks {
		if block.BlockType == db.ContentBlockType {
			content, err := ctx.dbClient.GetContentFromBlock(block.Id)
			if err != nil {
				return err
			}
			uiEditModuleBlock := NewUiEditModuleBlock(db.ContentBlockType, NewUiContent(content), []UiKnowledgePointDropdownItem{})
			uiBlocks = append(uiBlocks, uiEditModuleBlock)
		} else if block.BlockType == db.KnowledgePointBlockType {
			knowledgePoint, err := ctx.dbClient.GetKnowledgePointFromBlock(block.Id)
			if err != nil {
				return fmt.Errorf("Error getting knowledge point for block %d: %w", block.Id, err)
			}
			uiKnowledgePoints := make([]UiKnowledgePointDropdownItem, 0)
			for _, kp := range knowledgePoints {
				selected := kp.Id == knowledgePoint.Id
				uiKnowledgePoints = append(uiKnowledgePoints, NewUiKnowledgePointDropdownItem(kp.Id, kp.Name, selected))
			}
			uiEditModuleBlock := NewUiEditModuleBlock(db.KnowledgePointBlockType, UiContent{}, uiKnowledgePoints)
			uiBlocks = append(uiBlocks, uiEditModuleBlock)
		} else {
			return fmt.Errorf("invalid block type: %s", block.BlockType)
		}
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
	courseId          int64
	moduleId          int
	title             string
	description       string
	blockTypes        []string
	contents          []string
	knowledgePoints   []int64
}

func parseQuestions(r *http.Request) ([]string, [][]string, []int, []string, error) {
	questions := r.Form["question-title[]"]
	questionIdxs := r.Form["question-idx[]"]
	if len(questions) != len(questionIdxs) {
		return []string{}, [][]string{}, []int{}, []string{}, fmt.Errorf("Each question must have an index")
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
		correctChoiceIdxStr := r.Form.Get(fmt.Sprintf("correct-choice-%s", questionIdxs[i]))
		correctChoiceIdx, err := strconv.Atoi(correctChoiceIdxStr)
		if err != nil {
			return []string{}, [][]string{}, []int{}, []string{}, fmt.Errorf("Each question must have a correct choice")
		}
		for ; choiceIdx < len(choices); choiceIdx++ {
			choice := choices[choiceIdx]
			if choice == "end-choice" {
				choiceIdx++
				break
			}
			uiChoices = append(uiChoices, choice)
			choiceUiIdx, err := strconv.Atoi(choiceUiIdxs[choiceIdx])
			if err != nil {
				return []string{}, [][]string{}, []int{}, []string{}, fmt.Errorf("Error parsing choice index: %v", err)
			}
			if choiceUiIdx == correctChoiceIdx {
				correctChoicesByQuestion[i] = len(uiChoices) - 1
			}
		}
		uiQuestions[i] = question
		uiChoicesByQuestion[i] = uiChoices
	}
	return uiQuestions, uiChoicesByQuestion, correctChoicesByQuestion, explanations, nil
}

func parseEditModuleRequest(r *http.Request) (editModuleRequest, error) {
	courseIdInt, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return editModuleRequest{}, err
	}
	courseId := int64(courseIdInt)
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return editModuleRequest{}, err
	}
	err = r.ParseForm()
	if err != nil {
		return editModuleRequest{}, err
	}
	title := r.Form.Get("title")
	description := r.Form.Get("description")
	blockTypes := r.Form["block-type[]"]
	contents := r.Form["content-text[]"]
	knowledgePoints := r.Form["knowledge-point[]"]
	knowledgePointIds := make([]int64, 0)
	for _, kp := range knowledgePoints {
		kpId, err := strconv.Atoi(kp)
		if err != nil {
			return editModuleRequest{}, fmt.Errorf("Error parsing knowledge point id: %v", err)
		}
		knowledgePointIds = append(knowledgePointIds, int64(kpId))
	}
	return editModuleRequest{
		courseId,
		moduleId,
		title,
		description,
		blockTypes,
		contents,
		knowledgePointIds,
	}, nil
}

const MaxBlocks = 64
const MaxContentLength = 4096
const MaxQuestions = 64
const MaxQuestionLength = 2048
const MaxChoices = 16
const MaxChoiceLength = 1024

func validateQuestions(questions []string, choicesByQuestion [][]string, correctChoicesByQuestion []int, explanations []string) error {
	if len(questions) > MaxQuestions {
		return fmt.Errorf("Cannot have more than %d questions", MaxQuestions)
	}
	if len(questions) != len(choicesByQuestion) {
		return fmt.Errorf("Each question must have choices")
	}
	if len(questions) != len(correctChoicesByQuestion) {
		return fmt.Errorf("Each question must have a correct choice")
	}
	if len(questions) != len(explanations) {
		return fmt.Errorf("Each question must have an input (though the explanation itself can be empty)")
	}
	for i, question := range questions {
		if question == "" {
			return fmt.Errorf("Questions cannot be empty")
		}
		if len(question) > MaxQuestionLength {
			return fmt.Errorf("Questions cannot be longer than %d characters", MaxQuestionLength)
		}
		if len(choicesByQuestion[i]) == 0 {
			return fmt.Errorf("Questions must have at least one choice")
		}
		if len(choicesByQuestion[i]) > MaxChoices {
			return fmt.Errorf("Questions cannot have more than %d choices", MaxChoices)
		}
		for _, choice := range choicesByQuestion[i] {
			if choice == "" {
				return fmt.Errorf("Choices cannot be empty")
			}
			if len(choice) > MaxChoiceLength {
				return fmt.Errorf("Choices cannot be longer than %d characters", MaxChoiceLength)
			}
		}
	}
	return nil
}

func validateEditModuleRequest(req editModuleRequest) error {
	if len(req.blockTypes) > MaxBlocks {
		return fmt.Errorf("Cannot have more than %d blocks", MaxBlocks)
	}
	contentCount := 0
	kpCount := 0
	for _, blockType := range req.blockTypes {
		if db.BlockType(blockType) == db.ContentBlockType {
			contentCount += 1
		} else if db.BlockType(blockType) == db.KnowledgePointBlockType {
			kpCount += 1
		} else {
			return fmt.Errorf("invalid block type: %s", blockType)
		}
	}
	if contentCount != len(req.contents) {
		return fmt.Errorf("Each content block must have content")
	}
	if kpCount != len(req.knowledgePoints) {
		return fmt.Errorf("Each knowledge point block must have a knowledge point")
	}
	if req.title == "" {
		return fmt.Errorf("Title cannot be empty")
	}
	if len(req.title) > TitleMaxLength {
		return fmt.Errorf("Title cannot be longer than %d characters", TitleMaxLength)
	}
	if req.description == "" {
		return fmt.Errorf("Description cannot be empty")
	}
	if len(req.description) > DescriptionMaxLength {
		return fmt.Errorf("Description cannot be longer than %d characters", DescriptionMaxLength)
	}
	for _, content := range req.contents {
		if content == "" {
			return fmt.Errorf("Contents cannot be empty")
		}
		if len(content) > MaxContentLength {
			return fmt.Errorf("Contents cannot be longer than %d characters", MaxContentLength)
		}
	}
	return nil
}

func handleEditModule(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	req, err := parseEditModuleRequest(r)
	if err != nil {
		return fmt.Errorf("Error parsing edit module request: %v", err)
	}
	err = validateEditModuleRequest(req)
	if err != nil {
		return fmt.Errorf("Error validating edit module request: %v", err)
	}
	_, err = ctx.dbClient.GetModuleCourse(user.Id, req.moduleId)
	if err != nil {
		return fmt.Errorf("Module %d not found", req.moduleId)
	}
	for _, kpId := range req.knowledgePoints {
		_, err = ctx.dbClient.GetKnowledgePoint(int64(req.courseId), kpId)
		if err != nil {
			return fmt.Errorf("Knowledge point %d not found", kpId)
		}
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
	kpIdx := 0
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
		} else if db.BlockType(blockType) == db.KnowledgePointBlockType {
			err = db.InsertKnowledgePointBlock(tx, blockId, req.knowledgePoints[kpIdx])
			if err != nil {
				return err
			}
			kpIdx += 1
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
		block, err := ctx.dbClient.GetBlock(moduleVersion.Id, blockIdx)
		if err != nil {
			return fmt.Errorf("Error getting block %d for module %d: %v", blockIdx, moduleVersion.Id, err)
		}
		var uiBlock UiBlock
		switch block.BlockType {
		case db.ContentBlockType:
			content, err := ctx.dbClient.GetContentFromBlock(block.Id)
			if err != nil {
				return fmt.Errorf("Error getting content for block %d: %v", block.Id, err)
			}
			rendered, err := NewUiContentRendered(content)
			if err != nil {
				return fmt.Errorf("Error converting content for block %d: %v", block.Id, err)
			}
			uiBlock = NewUiBlockContent(rendered, blockIdx)
		case db.KnowledgePointBlockType:
			knowledgePoint, err := ctx.dbClient.GetKnowledgePointFromBlock(block.Id)
			if err != nil {
				return fmt.Errorf("Error getting knowledge point for block %d: %v", block.Id, err)
			}
			questions, err := ctx.dbClient.GetQuestionsForKnowledgePoint(knowledgePoint.Id)
			if err != nil {
				return fmt.Errorf("Error getting question for knowledge point %d: %v", knowledgePoint.Id, err)
			}
			// Just select a random question from the knowlegdge point
			// TODO: should load minimum effective dose once that's ready
			questionIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(questions))))
			if err != nil {
				return fmt.Errorf("Error getting random question index: %v", err)
			}
			question := questions[questionIdx.Int64()]
			uiQuestion, err := loadUiQuestion(ctx, question, user.Id)
			if err != nil {
				return fmt.Errorf("Error loading question for block %d: %v", block.Id, err)
			}
			uiBlock = NewUiBlockQuestion(uiQuestion, blockIdx)
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

func hasCycle(edges map[int][]int, root int) bool {
	visitPath := make(map[int]int) // curr -> parent
	curr := root
	visitPath[curr] = -1
	for curr != -1 {
		neighbors := edges[curr]
		neighborCount := len(neighbors)
		if neighborCount == 0 {
			parent := visitPath[curr]
			delete(visitPath, curr)
			curr = parent
		} else {
			next := neighbors[neighborCount-1]
			if _, ok := visitPath[next]; ok {
				return true
			}
			edges[curr] = neighbors[:neighborCount-1]
			visitPath[next] = curr
			curr = next
		}
	}
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
	if hasCycle(edges, req.moduleId) {
		return fmt.Errorf("Cannot create cycle in prereqs")
	}
	// Note: hasCycle modifies edges, so if we want to use it again
	// afterwards we'll need to pass a copy in.

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

func handleExportModule(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return err
	}
	_, err = ctx.dbClient.GetTeacherCourse(courseId, user.Id)
	if err != nil {
		return err
	}
	_, err = ctx.dbClient.GetModuleCourse(user.Id, moduleId)
	if err != nil {
		return err
	}
	moduleVersion, err := ctx.dbClient.GetLatestModuleVersion(moduleId)
	if err != nil {
		return err
	}
	blocks, err := ctx.dbClient.GetBlocks(moduleVersion.Id)
	if err != nil {
		return err
	}
	metadataStr := func(text string) string {
		return fmt.Sprintf("\n[//]: # (%s)", text)
	}
	textPieces := make([]string, 0)
	textPieces = append(textPieces, fmt.Sprintf("---\ntitle: %s\ndescription: %s\n---\n", moduleVersion.Title, moduleVersion.Description))
	for _, block := range blocks {
		if block.BlockType == db.ContentBlockType {
			content, err := ctx.dbClient.GetContentFromBlock(block.Id)
			if err != nil {
				return err
			}
			textPieces = append(textPieces, metadataStr("content"))
			textPieces = append(textPieces, content.Content)
		} else if block.BlockType == db.KnowledgePointBlockType {
			knowledgePoint, err := ctx.dbClient.GetKnowledgePointFromBlock(block.Id)
			if err != nil {
				return err
			}
			questions, err := ctx.dbClient.GetLatestQuestionsForKnowledgePoint(knowledgePoint.Id)
			if err != nil {
				return err
			}
			// TODO: handle multiple questions later
			question := questions[0]
			questionContent, err := ctx.dbClient.GetContent(question.ContentId)
			if err != nil {
				return err
			}
			choices, err := ctx.dbClient.GetChoicesForQuestion(question.Id)
			if err != nil {
				return err
			}
			choiceContents := make([]db.Content, 0)
			for _, choice := range choices {
				choiceContent, err := ctx.dbClient.GetContent(choice.ContentId)
				if err != nil {
					return err
				}
				choiceContents = append(choiceContents, choiceContent)
			}
			explanation, err := ctx.dbClient.GetExplanationForQuestion(question.Id)
			if err != nil {
				return err
			}
			textPieces = append(textPieces, metadataStr("question"))
			textPieces = append(textPieces, questionContent.Content)
			for i, choice := range choices {
				metadata := "choice"
				if choice.Correct {
					metadata += " correct"
				}
				textPieces = append(textPieces, metadataStr(metadata))
				textPieces = append(textPieces, choiceContents[i].Content)
			}
			if explanation.Content != "" {
				textPieces = append(textPieces, metadataStr("explanation"))
				textPieces = append(textPieces, explanation.Content)
			}
		} else {
			return fmt.Errorf("invalid block type: %s", block.BlockType)
		}
	}
	text := strings.Join(textPieces, "\n")
	return ctx.renderer.RenderExportedModule(w, text)
}

// Knowledge Points

func handleKnowledgePointPage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	courseIdInt, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	courseId := int64(courseIdInt)
	course, err := ctx.dbClient.GetTeacherCourse(int(courseId), user.Id)
	if err != nil {
		return err
	}

	knowledgePoints, err := ctx.dbClient.GetKnowledgePoints(courseId)
	uiKnowledgePoints := make([]UiKnowledgePointListItem, 0)
	for _, knowledgePoint := range knowledgePoints {
		uiKnowledgePoints = append(uiKnowledgePoints, NewUiKnowledgePointListItem(knowledgePoint))
	}
	pageArgs := UiKnowledgePointListPageArgs{
		CourseId:        courseId,
		CourseTitle:     course.Title,
		KnowledgePoints: uiKnowledgePoints,
	}

	return ctx.renderer.RenderKnowledgePointListPage(w, pageArgs)
}

func handleCreateKnowledgePointPage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	courseIdInt, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	courseId := int64(courseIdInt)
	course, err := ctx.dbClient.GetTeacherCourse(int(courseId), user.Id)
	if err != nil {
		return err
	}
	pageArgs := UiKnowledgePointPageArgs{
		CourseId:       courseId,
		CourseTitle:    course.Title,
		KnowledgePoint: NewUiKnowledgePointEmpty(),
	}
	return ctx.renderer.RenderCreateKnowledgePointPage(w, pageArgs)
}

func handleEditKnowledgePointPage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	courseIdInt, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	courseId := int64(courseIdInt)
	course, err := ctx.dbClient.GetTeacherCourse(int(courseId), user.Id)
	if err != nil {
		return err
	}
	kpIdInt, err := strconv.Atoi(r.PathValue("kpId"))
	if err != nil {
		return err
	}
	kpId := int64(kpIdInt)
	knowledgePoint, err := ctx.dbClient.GetKnowledgePoint(courseId, kpId)
	if err != nil {
		return err
	}
	questions, err := ctx.dbClient.GetLatestQuestionsForKnowledgePoint(knowledgePoint.Id)
	if err != nil {
		return err
	}
	uiQuestions := make([]UiQuestion, 0)
	for _, question := range questions {
		questionContent, choices, choiceContents, explanation, err := getQuestion(ctx, question)
		if err != nil {
			return err
		}
		uiQuestion := NewUiQuestionEdit(question, questionContent, choices, choiceContents, explanation)
		uiQuestions = append(uiQuestions, uiQuestion)
	}
	uiKp := NewUiKnowledgePoint(kpId, knowledgePoint.Name, uiQuestions)
	pageArgs := UiKnowledgePointPageArgs{
		CourseId:       courseId,
		CourseTitle:    course.Title,
		KnowledgePoint: uiKp,
	}
	return ctx.renderer.RenderCreateKnowledgePointPage(w, pageArgs)
}

type createKnowledgePointRequest struct {
	courseId          int64
	name              string
	questions         []string
	choicesByQuestion [][]string
	correctChoiceIdxs []int
	explanations      []string
}

func parseCreateKnowledgePointRequest(r *http.Request) (createKnowledgePointRequest, error) {
	courseIdInt, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return createKnowledgePointRequest{}, err
	}
	courseId := int64(courseIdInt)
	err = r.ParseForm()
	if err != nil {
		return createKnowledgePointRequest{}, err
	}
	name := r.Form.Get("kp-name")
	uiQuestions, uiChoicesByQuestion, correctChoicesByQuestion, explanations, err := parseQuestions(r)
	if err != nil {
		return createKnowledgePointRequest{}, err
	}
	return createKnowledgePointRequest{
		courseId,
		name,
		uiQuestions,
		uiChoicesByQuestion,
		correctChoicesByQuestion,
		explanations,
	}, nil
}

func validateCreateKnowledgePointRequest(req createKnowledgePointRequest) error {
	if req.name == "" {
		return fmt.Errorf("Name cannot be empty")
	}
	err := validateQuestions(req.questions, req.choicesByQuestion, req.correctChoiceIdxs, req.explanations)
	if err != nil {
		return fmt.Errorf("Error validating questions: %v", err)
	}
	if len(req.questions) == 0 {
		return fmt.Errorf("Knowledge point must have at least one question")
	}
	return nil
}

func handleCreateKnowledgePoint(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	req, err := parseCreateKnowledgePointRequest(r)
	if err != nil {
		return fmt.Errorf("Error parsing create knowledge point request: %v", err)
	}
	err = validateCreateKnowledgePointRequest(req)
	if err != nil {
		return fmt.Errorf("Error validating create knowledge point request: %v", err)
	}
	_, err = ctx.dbClient.GetTeacherCourse(int(req.courseId), user.Id)
	if err != nil {
		return err
	}
	tx, err := ctx.dbClient.Begin()
	defer tx.Rollback()
	if err != nil {
		return err
	}
	knowledgePoint, err := db.InsertKnowledgePoint(tx, req.courseId, req.name)
	if err != nil {
		return err
	}

	for i, _ := range req.questions {
		err = db.InsertQuestion(tx, knowledgePoint.Id, req.questions[i], req.choicesByQuestion[i], req.correctChoiceIdxs[i], req.explanations[i])
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	w.Header().Add("HX-Redirect", fmt.Sprintf("/teacher/course/%d/knowledge-point", req.courseId))
	return nil
}

func handleEditKnowledgePoint(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	req, err := parseCreateKnowledgePointRequest(r)
	if err != nil {
		return fmt.Errorf("Error parsing create knowledge point request: %v", err)
	}
	err = validateCreateKnowledgePointRequest(req)
	if err != nil {
		return fmt.Errorf("Error validating create knowledge point request: %v", err)
	}
	_, err = ctx.dbClient.GetTeacherCourse(int(req.courseId), user.Id)
	if err != nil {
		return err
	}
	kpIdInt, err := strconv.Atoi(r.PathValue("kpId"))
	if err != nil {
		return err
	}
	kpId := int64(kpIdInt)
	tx, err := ctx.dbClient.Begin()
	defer tx.Rollback()
	if err != nil {
		return err
	}
	knowledgePoint, err := db.UpdateKnowledgePoint(tx, kpId, req.courseId, req.name)
	if err != nil {
		return err
	}

	err = db.DeleteUnansweredQuestionsForKnowledgePoint(tx, knowledgePoint.Id)
	if err != nil {
		return err
	}
	err = db.MarkQuestionsOld(tx, knowledgePoint.Id)
	if err != nil {
		return err
	}
	for i, _ := range req.questions {
		err = db.InsertQuestion(tx, knowledgePoint.Id, req.questions[i], req.choicesByQuestion[i], req.correctChoiceIdxs[i], req.explanations[i])
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return ctx.renderer.RenderKnowledgePointEdited(w)
}

func handleDeleteKnowledgePoint(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	return nil
}
