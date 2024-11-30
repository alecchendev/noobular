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

func getTakeModule(w http.ResponseWriter, r *http.Request, ctx HandlerContext, userId int64) (UiTakeModule, error) {
	// courseId, err := strconv.Atoi(r.PathValue("courseId"))
	// if err != nil {
	//	return UiTakeModule{}, err
	// }
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return UiTakeModule{}, err
	}
	blockIdx, err := strconv.Atoi(r.PathValue("blockIdx"))
	if err != nil {
		return UiTakeModule{}, err
	}
	module, err := ctx.dbClient.GetModule(moduleId)
	if err != nil {
		return UiTakeModule{}, err
	}
	// TODO: add restrictions, i.e. you cannot take a question before a previous one
	blockCount, err := ctx.dbClient.GetBlockCount(moduleId)
	if err != nil {
		return UiTakeModule{}, err
	}
	if blockIdx >= blockCount {
		return UiTakeModule{}, fmt.Errorf("Block index %d is out of bounds (>=%d) for module %d", blockIdx, blockCount, moduleId)
	}
	block, err := ctx.dbClient.GetBlock(moduleId, blockIdx)
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
		correctChoiceId := -1
		for _, choice := range choices {
			if choice.Correct {
				correctChoiceId = choice.Id
				break
			}
		}
		explanationContent, err := ctx.dbClient.GetExplanationForQuestion(question.Id)
		var buf bytes.Buffer
		if err := goldmark.Convert([]byte(explanationContent.Content), &buf); err != nil {
			return UiTakeModule{}, err
		}
		explanation := template.HTML(buf.String())
		return UiTakeModule{
			Module:          NewUiModule(module),
			BlockType:       string(db.QuestionBlockType),
			Content:         template.HTML(""),
			BlockCount:      blockCount,
			BlockIndex:      blockIdx,
			ChosenChoiceId:  choiceId,
			CorrectChoiceId: correctChoiceId,
			Question:        NewUiQuestion(question, choices, explanationContent),
			Explanation:     explanation,
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
		return UiTakeModule{
			Module:          NewUiModule(module),
			BlockType:       string(db.ContentBlockType),
			Content:         template.HTML(buf.String()),
			BlockCount:      blockCount,
			BlockIndex:      blockIdx,
			ChosenChoiceId:  -1,
			CorrectChoiceId: -1,
			Question:        EmptyQuestion(),
			Explanation:     template.HTML(""),
		}, nil
	} else {
		return UiTakeModule{}, fmt.Errorf("Unknown block type %s", block.BlockType)
	}
}

func handleTakeModulePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, userId int64) error {
	module, err := getTakeModule(w, r, ctx, userId)
	if err != nil {
		return err
	}
	return ctx.renderer.RenderTakeModulePage(w, module)
}

func handleTakeModule(w http.ResponseWriter, r *http.Request, ctx HandlerContext, userId int64) error {
	module, err := getTakeModule(w, r, ctx, userId)
	if err != nil {
		return err
	}
	return ctx.renderer.RenderTakeModule(w, module)
}

func handleAnswerQuestion(w http.ResponseWriter, r *http.Request, ctx HandlerContext, userId int64) error {
	uiTakeModule, err := getTakeModule(w, r, ctx, userId)
	if err != nil {
		return err
	}
	err = r.ParseForm()
	if err != nil {
		return err
	}
	choiceId, err := strconv.Atoi(r.Form.Get("choice"))
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	err = ctx.dbClient.StoreAnswer(userId, uiTakeModule.Question.Id, choiceId)
	if err != nil {
		return err
	}

	return ctx.renderer.RenderQuestionSubmitted(w, UiSubmittedAnswer{
		Module:          uiTakeModule.Module,
		BlockCount:      uiTakeModule.BlockCount,
		BlockIndex:      uiTakeModule.BlockIndex,
		ChosenChoiceId:  choiceId,
		CorrectChoiceId: uiTakeModule.CorrectChoiceId,
		Question:        uiTakeModule.Question,
		Explanation:     uiTakeModule.Explanation,
	})
}