package internal

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/yuin/goldmark"

	"noobular/internal/db"
)

// Student page

func handleStudentPage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, userId int64) error {
	user, err := ctx.dbClient.GetUser(userId)
	if err != nil {
		return err
	}
	courses, err := ctx.dbClient.GetEnrolledCourses(userId)
	uiCourses := make([]UiCourse, len(courses))
	for i, course := range courses {
		uiCourses[i] = NewUiCourse(course, []UiModule{})
	}
	return ctx.renderer.RenderStudentPage(w, StudentPageArgs{user.Username, uiCourses})
}

// Student course page

func handleStudentCoursePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, userId int64) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	_, err = ctx.dbClient.GetEnrollment(userId, courseId)
	if err != nil {
		return err
	}
	user, err := ctx.dbClient.GetUser(userId)
	if err != nil {
		return err
	}
	course, err := ctx.dbClient.GetCourse(courseId)
	if err != nil {
		return err
	}
	moduleVersions, err := ctx.dbClient.GetLatestModuleVersionsForCourse(course.Id, true)
	if err != nil {
		return err
	}
	uiModules := make([]UiModule, len(moduleVersions))
	for j, moduleVersion := range moduleVersions {
		uiModules[j] = NewUiModule(courseId, moduleVersion)
	}
	return ctx.renderer.RenderStudentCoursePage(w, StudentCoursePageArgs{
		Username:    user.Username,
		Course:      NewUiCourse(course, uiModules),
	})
}

// Take course

func handleTakeCourse(w http.ResponseWriter, r *http.Request, ctx HandlerContext, userId int64) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	_, err = ctx.dbClient.InsertEnrollment(userId, courseId)
	if err != nil {
		return err
	}
	w.Header().Add("HX-Redirect", fmt.Sprintf("/student/course/%d", courseId))
	return nil
}

// Take module page

type takeModuleRequest struct {
	moduleId int
	blockIdx int
}

func parseTakeModuleRequest(r *http.Request) (takeModuleRequest, error) {
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return takeModuleRequest{}, err
	}
	blockIdx, err := strconv.Atoi(r.PathValue("blockIdx"))
	if err != nil {
		return takeModuleRequest{}, err
	}
	return takeModuleRequest{moduleId, blockIdx}, nil
}

func getModule(ctx HandlerContext, moduleId int, userId int64) (UiModule, db.Visit, int, error) {
	module, err := ctx.dbClient.GetModule(moduleId)
	if err != nil {
		return UiModule{}, db.Visit{}, 0, err
	}
	visit, err := ctx.dbClient.GetOrCreateVisit(userId, moduleId)
	if err != nil {
		return UiModule{}, db.Visit{}, 0, err
	}
	moduleVersion, err := ctx.dbClient.GetModuleVersion(visit.ModuleVersionId)
	if err != nil {
		return UiModule{}, db.Visit{}, 0, err
	}
	blockCount, err := ctx.dbClient.GetBlockCount(visit.ModuleVersionId)
	if err != nil {
		return UiModule{}, db.Visit{}, 0, err
	}
	return NewUiModule(module.CourseId, moduleVersion), visit, blockCount, nil
}

func getBlock(ctx HandlerContext, moduleVersionId int64, blockIdx int, userId int64) (UiBlock, error) {
	block, err := ctx.dbClient.GetBlock(moduleVersionId, blockIdx)
	if err != nil {
		return UiBlock{}, fmt.Errorf("Error getting block %d for module %d: %v", blockIdx, moduleVersionId, err)
	}
	// TODO: use a html sanitizer like blue monday?
	if block.BlockType == db.QuestionBlockType {
		question, err := ctx.dbClient.GetQuestionFromBlock(block.Id)
		if err != nil {
			return UiBlock{}, fmt.Errorf("Error getting question for block %d: %v", block.Id, err)
		}
		choiceId, err := ctx.dbClient.GetAnswer(userId, question.Id)
		if err != nil {
			return UiBlock{}, fmt.Errorf("Error getting answer for question %d: %v", question.Id, err)
		}
		choices, err := ctx.dbClient.GetChoicesForQuestion(question.Id)
		explanationContent, err := ctx.dbClient.GetExplanationForQuestion(question.Id)
		if err != nil {
			return UiBlock{}, fmt.Errorf("Error getting explanation for question %d: %v", question.Id, err)
		}
		var buf bytes.Buffer
		if err := goldmark.Convert([]byte(explanationContent.Content), &buf); err != nil {
			return UiBlock{}, fmt.Errorf("Error converting explanation content for question %d: %v", question.Id, err)
		}
		explanation := template.HTML(buf.String())
		var uiQuestion UiQuestion
		if choiceId == -1 {
			uiQuestion = NewUiQuestionTake(question, choices, NewUiContentRendered(explanationContent, explanation))
		} else {
			uiQuestion = NewUiQuestionAnswered(question, choices, choiceId, NewUiContentRendered(explanationContent, explanation))
		}
		uiBlock := NewUiBlockQuestion(uiQuestion, blockIdx)
		return uiBlock, nil
	} else if block.BlockType == db.ContentBlockType {
		content, err := ctx.dbClient.GetContentFromBlock(block.Id)
		if err != nil {
			return UiBlock{}, fmt.Errorf("Error getting content for block %d: %v", block.Id, err)
		}
		var buf bytes.Buffer
		if err := goldmark.Convert([]byte(content.Content), &buf); err != nil {
			return UiBlock{}, fmt.Errorf("Error converting content for block %d: %v", block.Id, err)
		}
		uiBlock := NewUiBlockContent(NewUiContentRendered(content, template.HTML(buf.String())), blockIdx)
		return uiBlock, nil
	} else {
		return UiBlock{}, fmt.Errorf("Unknown block type %s", block.BlockType)
	}
}

func handleTakeModulePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, userId int64) error {
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return err
	}
	module, visit, blockCount, err := getModule(ctx, moduleId, userId)
	if err != nil {
		return fmt.Errorf("Error getting module %d: %v", moduleId, err)
	}
	if visit.BlockIndex > blockCount {
		return fmt.Errorf("Block index %d is out of bounds (>=%d) for module %d", visit.BlockIndex, blockCount, moduleId)
	}
	nBlocks := min(visit.BlockIndex + 1, blockCount)
	uiBlocks := make([]UiBlock, nBlocks)
	for blockIdx := 0; blockIdx < nBlocks; blockIdx++ {
		uiBlock, err := getBlock(ctx, visit.ModuleVersionId, blockIdx, userId)
		if err != nil {
			return fmt.Errorf("Error getting block %d for module %d: %v", blockIdx, moduleId, err)
		}
		uiBlocks[blockIdx] = uiBlock
	}
	uiModule := UiTakeModulePage{
		Module:          module,
		Blocks:		 uiBlocks,
		BlockCount:      blockCount,
		VisitIndex:      visit.BlockIndex,
	}
	return ctx.renderer.RenderTakeModulePage(w, uiModule)
}

func getTakeModule(req takeModuleRequest, ctx HandlerContext, userId int64) (UiTakeModule, db.Visit, error) {
	module, visit, blockCount, err := getModule(ctx, req.moduleId, userId)
	if err != nil {
		return UiTakeModule{}, db.Visit{}, err
	}
	if req.blockIdx >= blockCount {
		return UiTakeModule{}, db.Visit{}, fmt.Errorf("Block index %d is out of bounds (>=%d) for module %d", req.blockIdx, blockCount, req.moduleId)
	}
	if req.blockIdx > visit.BlockIndex + 1 {
		return UiTakeModule{}, db.Visit{}, fmt.Errorf("Block index %d is ahead of visit block index %d for module %d", req.blockIdx, visit.BlockIndex, req.moduleId)
	}
	uiBlock, err := getBlock(ctx, visit.ModuleVersionId, req.blockIdx, userId)
	if err != nil {
		return UiTakeModule{}, db.Visit{}, err
	}
	return UiTakeModule{
		Module:          module,
		Block:		 uiBlock,
		BlockCount:      blockCount,
		VisitIndex:      visit.BlockIndex,
	}, visit, nil
}

func handleTakeModule(w http.ResponseWriter, r *http.Request, ctx HandlerContext, userId int64) error {
	req, err := parseTakeModuleRequest(r)
	if err != nil {
		return err
	}
	module, visit, err := getTakeModule(req, ctx, userId)
	if err != nil {
		return err
	}
	err = ctx.dbClient.UpdateVisit(userId, visit.ModuleVersionId, req.blockIdx)
	if err != nil {
		return err
	}
	module.VisitIndex = req.blockIdx
	return ctx.renderer.RenderTakeModule(w, module)
}

func handleAnswerQuestion(w http.ResponseWriter, r *http.Request, ctx HandlerContext, userId int64) error {
	req, err := parseTakeModuleRequest(r)
	if err != nil {
		return err
	}
	uiTakeModule, _, err := getTakeModule(req, ctx, userId)
	if err != nil {
		return err
	}
	if uiTakeModule.Block.BlockType != db.QuestionBlockType {
		return fmt.Errorf("Tried to submit answer, but block at index %d for module %d is not a question block", req.blockIdx, req.moduleId)
	}
	err = r.ParseForm()
	if err != nil {
		return err
	}
	choiceId, err := strconv.Atoi(r.Form.Get("choice"))
	if err != nil {
		return err
	}
	err = ctx.dbClient.StoreAnswer(userId, uiTakeModule.Block.Question.Id, choiceId)
	if err != nil {
		return err
	}
	for i, choice := range uiTakeModule.Block.Question.Choices {
		if choice.Id == choiceId {
			uiTakeModule.Block.Question.Choices[i].Chosen = true
			break
		}
	}
	return ctx.renderer.RenderQuestionSubmitted(w, uiTakeModule)
}

func handleCompleteModule(w http.ResponseWriter, r *http.Request, ctx HandlerContext, userId int64) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return err
	}
	visit, err := ctx.dbClient.GetVisit(userId, moduleId)
	if err != nil {
		return err
	}
	blockCount, err := ctx.dbClient.GetBlockCount(visit.ModuleVersionId)
	if err != nil {
		return err
	}
	if visit.BlockIndex < blockCount - 1 {
		return fmt.Errorf("Tried to complete module %d, but only at block index %d", moduleId, visit.BlockIndex)
	}
	err = ctx.dbClient.UpdateVisit(userId, visit.ModuleVersionId, blockCount)
	if err != nil {
		return err
	}
	w.Header().Add("HX-Redirect", fmt.Sprintf("/student/course/%d", courseId))
	return nil
}
