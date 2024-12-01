package internal

import (
	"bytes"
	"database/sql"
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
	modules, err := ctx.dbClient.GetModules(course.Id, true)
	if err != nil {
		return err
	}
	uiModules := make([]UiModuleStudent, len(modules))
	for j, module := range modules {
		blockCount, err := ctx.dbClient.GetBlockCount(module.Id)
		if err != nil {
			return err
		}
		visit, err := ctx.dbClient.GetVisit(userId, module.Id)
		var nextBlockIdx int
		if err == sql.ErrNoRows {
			nextBlockIdx = -1
		} else if err != nil {
			return err
		}
		nextBlockIdx = visit.BlockIndex
		uiModules[j] = UiModuleStudent{
			module.Id,
			module.CourseId,
			module.Title,
			module.Description,
			blockCount,
			nextBlockIdx,
		}
	}
	return ctx.renderer.RenderStudentCoursePage(w, StudentCoursePageArgs{
		Username:    user.Username,
		Course:      UiCourseStudent{course.Id, course.Title, course.Description, uiModules},
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
	blockCount, err := ctx.dbClient.GetBlockCount(moduleId)
	if err != nil {
		return UiModule{}, db.Visit{}, 0, err
	}
	visit, err := ctx.dbClient.GetOrCreateVisit(userId, moduleId)
	if err != nil {
		return UiModule{}, db.Visit{}, 0, err
	}
	return NewUiModule(module), visit, blockCount, nil
}

func getBlock(ctx HandlerContext, moduleId int, blockIdx int, userId int64) (UiBlock, error) {
	block, err := ctx.dbClient.GetBlock(moduleId, blockIdx)
	if err != nil {
		return UiBlock{}, err
	}
	// TODO: use a html sanitizer like blue monday?
	if block.BlockType == db.QuestionBlockType {
		question, err := ctx.dbClient.GetQuestionFromBlock(block.Id)
		if err != nil {
			return UiBlock{}, err
		}
		choiceId, err := ctx.dbClient.GetAnswer(userId, question.Id)
		if err != nil {
			return UiBlock{}, err
		}
		choices, err := ctx.dbClient.GetChoicesForQuestion(question.Id)
		explanationContent, err := ctx.dbClient.GetExplanationForQuestion(question.Id)
		var buf bytes.Buffer
		if err := goldmark.Convert([]byte(explanationContent.Content), &buf); err != nil {
			return UiBlock{}, err
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
			return UiBlock{}, err
		}
		var buf bytes.Buffer
		if err := goldmark.Convert([]byte(content.Content), &buf); err != nil {
			return UiBlock{}, err
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
		return err
	}
	if visit.BlockIndex > blockCount {
		return fmt.Errorf("Block index %d is out of bounds (>=%d) for module %d", visit.BlockIndex, blockCount, moduleId)
	}
	nBlocks := min(visit.BlockIndex + 1, blockCount)
	uiBlocks := make([]UiBlock, nBlocks)
	for blockIdx := 0; blockIdx < nBlocks; blockIdx++ {
		uiBlock, err := getBlock(ctx, moduleId, blockIdx, userId)
		if err != nil {
			return err
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

func getTakeModule(req takeModuleRequest, ctx HandlerContext, userId int64) (UiTakeModule, error) {
	module, visit, blockCount, err := getModule(ctx, req.moduleId, userId)
	if err != nil {
		return UiTakeModule{}, err
	}
	if req.blockIdx >= blockCount {
		return UiTakeModule{}, fmt.Errorf("Block index %d is out of bounds (>=%d) for module %d", req.blockIdx, blockCount, req.moduleId)
	}
	if req.blockIdx > visit.BlockIndex + 1 {
		return UiTakeModule{}, fmt.Errorf("Block index %d is ahead of visit block index %d for module %d", req.blockIdx, visit.BlockIndex, req.moduleId)
	}
	uiBlock, err := getBlock(ctx, req.moduleId, req.blockIdx, userId)
	if err != nil {
		return UiTakeModule{}, err
	}
	return UiTakeModule{
		Module:          module,
		Block:		 uiBlock,
		BlockCount:      blockCount,
		VisitIndex:      visit.BlockIndex,
	}, nil
}

func handleTakeModule(w http.ResponseWriter, r *http.Request, ctx HandlerContext, userId int64) error {
	req, err := parseTakeModuleRequest(r)
	if err != nil {
		return err
	}
	module, err := getTakeModule(req, ctx, userId)
	if err != nil {
		return err
	}
	err = ctx.dbClient.UpdateVisit(userId, req.moduleId, req.blockIdx)
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
	uiTakeModule, err := getTakeModule(req, ctx, userId)
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
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return err
	}
	visit, err := ctx.dbClient.GetVisit(userId, moduleId)
	if err != nil {
		return err
	}
	blockCount, err := ctx.dbClient.GetBlockCount(moduleId)
	if err != nil {
		return err
	}
	if visit.BlockIndex < blockCount - 1 {
		return fmt.Errorf("Tried to complete module %d, but only at block index %d", moduleId, visit.BlockIndex)
	}
	err = ctx.dbClient.UpdateVisit(userId, moduleId, blockCount)
	if err != nil {
		return err
	}
	w.Header().Add("HX-Redirect", "/student/course")
	return nil
}
