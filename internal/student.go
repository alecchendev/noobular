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
	return ctx.renderer.RenderStudentPage(w, StudentPageArgs{user.Username})
}

// Courses page

func handleStudentCoursesPage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, userId int64) error {
	courses, err := ctx.dbClient.GetCourses(-1, true)
	if err != nil {
		return err
	}
	uiCourses := make([]UiCourseStudent, len(courses))
	for i, course := range courses {
		modules, err := ctx.dbClient.GetModules(course.Id, true)
		if err != nil {
			return err
		}
		uiModules := make([]UiModuleStudent, len(modules))
		for j, module := range modules {
			questionCount, err := ctx.dbClient.GetQuestionCount(module.Id)
			if err != nil {
				return err
			}
			// TODO: this is broken. We should store the latest
			// block they got to.
			nextUnansweredQuestionIdx, err := ctx.dbClient.GetNextUnansweredQuestionIdx(userId, module.Id)
			if err != nil {
				return err
			}
			uiModules[j] = UiModuleStudent{
				module.Id,
				module.CourseId,
				module.Title,
				module.Description,
				questionCount,
				nextUnansweredQuestionIdx,
			}
		}
		uiCourses[i] = UiCourseStudent{course.Id, course.Title, course.Description, uiModules}
	}
	return ctx.renderer.RenderStudentCoursePage(w, uiCourses)
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

func getTakeModule(req takeModuleRequest, ctx HandlerContext, userId int64) (UiTakeModule, error) {
	module, err := ctx.dbClient.GetModule(req.moduleId)
	if err != nil {
		return UiTakeModule{}, err
	}
	// TODO: add restrictions, i.e. you cannot take a question before a previous one
	blockCount, err := ctx.dbClient.GetBlockCount(req.moduleId)
	if err != nil {
		return UiTakeModule{}, err
	}
	if req.blockIdx >= blockCount {
		return UiTakeModule{}, fmt.Errorf("Block index %d is out of bounds (>=%d) for module %d", req.blockIdx, blockCount, req.moduleId)
	}
	block, err := ctx.dbClient.GetBlock(req.moduleId, req.blockIdx)
	if err != nil {
		return UiTakeModule{}, err
	}
	// TODO: use a html sanitizer like blue monday?
	if block.BlockType == db.QuestionBlockType {
		question, err := ctx.dbClient.GetQuestionFromBlock(block.Id)
		if err != nil {
			return UiTakeModule{}, err
		}
		choiceId, err := ctx.dbClient.GetAnswer(userId, question.Id)
		if err != nil {
			return UiTakeModule{}, err
		}
		choices, err := ctx.dbClient.GetChoicesForQuestion(question.Id)
		explanationContent, err := ctx.dbClient.GetExplanationForQuestion(question.Id)
		var buf bytes.Buffer
		if err := goldmark.Convert([]byte(explanationContent.Content), &buf); err != nil {
			return UiTakeModule{}, err
		}
		explanation := template.HTML(buf.String())
		var uiQuestion UiQuestion
		if choiceId == -1 {
			uiQuestion = NewUiQuestionTake(question, choices, NewUiContentRendered(explanationContent, explanation))
		} else {
			uiQuestion = NewUiQuestionAnswered(question, choices, choiceId, NewUiContentRendered(explanationContent, explanation))
		}
		uiBlock := NewUiBlockQuestion(uiQuestion)
		return UiTakeModule{
			Module:          NewUiModule(module),
			Block:		 uiBlock,
			BlockCount:      blockCount,
			BlockIndex:      req.blockIdx,
		}, nil
	} else if block.BlockType == db.ContentBlockType {
		content, err := ctx.dbClient.GetContentFromBlock(block.Id)
		if err != nil {
			return UiTakeModule{}, err
		}
		var buf bytes.Buffer
		if err := goldmark.Convert([]byte(content.Content), &buf); err != nil {
			return UiTakeModule{}, err
		}
		uiBlock := NewUiBlockContent(NewUiContentRendered(content, template.HTML(buf.String())))
		return UiTakeModule{
			Module:          NewUiModule(module),
			Block:		 uiBlock,
			BlockCount:      blockCount,
			BlockIndex:      req.blockIdx,
		}, nil
	} else {
		return UiTakeModule{}, fmt.Errorf("Unknown block type %s", block.BlockType)
	}
}

func handleTakeModulePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, userId int64) error {
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return err
	}
	module, err := getTakeModule(takeModuleRequest{moduleId, 0}, ctx, userId)
	if err != nil {
		return err
	}
	return ctx.renderer.RenderTakeModulePage(w, module)
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
