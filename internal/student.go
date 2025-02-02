package internal

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"time"

	"noobular/internal/db"
)

// Student page

func handleStudentPage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	courses, err := ctx.dbClient.GetEnrolledCourses(user.Id)
	if err != nil {
		return err
	}
	uiCourses := make([]UiCourse, len(courses))
	for i, course := range courses {
		uiCourses[i] = NewUiCourse(course, []UiModule{})
	}
	return ctx.renderer.RenderStudentPage(w, StudentPageArgs{user.Username, uiCourses})
}

// Student course page

func handleStudentCoursePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	_, err = ctx.dbClient.GetEnrollment(user.Id, courseId)
	if err != nil {
		return err
	}
	course, err := ctx.dbClient.GetCourse(courseId)
	if err != nil {
		return err
	}
	modules, err := ctx.dbClient.GetModules(course.Id)
	if err != nil {
		return err
	}
	visitMap := make(map[int]db.Visit) // moduleId -> visit
	versionMap := make(map[int]db.ModuleVersion) // moduleId -> module version
	blockCountMap := make(map[int]int) // moduleId -> block count
	for _, module := range modules {
		visit, err := ctx.dbClient.GetVisit(user.Id, module.Id)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil {
			visitMap[module.Id] = visit
		}
		var moduleVersion db.ModuleVersion
		if err == sql.ErrNoRows {
			moduleVersion, err = ctx.dbClient.GetLatestModuleVersion(module.Id)
		} else {
			moduleVersion, err = ctx.dbClient.GetModuleVersion(visit.ModuleVersionId)
		}
		if err != nil {
			return err
		}
		versionMap[module.Id] = moduleVersion
		blockCount, err := ctx.dbClient.GetBlockCount(moduleVersion.Id)
		if err != nil {
			return err
		}
		blockCountMap[module.Id] = blockCount
	}

	uiModules := make([]UiModule, 0)
	totalPoints := 0
	for _, module := range modules {
		prereqs, err := ctx.dbClient.GetPrereqs(module.Id)
		if err != nil {
			return err
		}
		completedAllPrereqs := true
		for _, prereq := range prereqs {
			visit, ok := visitMap[prereq.PrereqModuleId]
			if !ok || visit.BlockIndex != blockCountMap[prereq.PrereqModuleId] {
				completedAllPrereqs = false
				break
			}
		}
		if !completedAllPrereqs {
			log.Printf("Skipping module because not all prereqs are completed: %d", module.Id)
			continue
		}

		visit, _ := visitMap[module.Id]
		moduleVersion, _ := versionMap[module.Id]
		blockCount := blockCountMap[module.Id]
		if blockCount == 0 {
			log.Printf("Skipping module because it has no blocks: %d %s", module.Id, moduleVersion.Title)
			continue
		}
		point, err := ctx.dbClient.GetPoint(user.Id, module.Id)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		pointCount := 0
		if err == nil {
			pointCount = point.Count
		}
		totalPoints += pointCount
		completed := visit.BlockIndex == blockCount

		uiModules = append(uiModules, NewUiModuleStudent(course.Id, moduleVersion, blockCount, completed, point.CreatedAt, pointCount))
	}
	sort.Slice(uiModules, func(i, j int) bool {
		if !uiModules[i].Completed { return true }
		if !uiModules[j].Completed { return false }
		return uiModules[i].CompletedAt.After(uiModules[j].CompletedAt)
	})
	return ctx.renderer.RenderStudentCoursePage(w, StudentCoursePageArgs{
		Username:    user.Username,
		Course:      NewUiCourse(course, uiModules),
		TotalPoints: totalPoints,
	})
}

// Take course

func handleTakeCourse(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	tx, err := ctx.dbClient.Begin()
	defer tx.Rollback()
	if err != nil {
		return err
	}
	course, err := db.GetCourse(tx, courseId)
	if err != nil {
		return err
	}
	if !course.Public {
		return fmt.Errorf("Cannot enroll in private course.")
	}
	_, err = db.InsertEnrollment(tx, user.Id, courseId)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	w.Header().Add("HX-Redirect", fmt.Sprintf("/student/course/%d", courseId))
	return nil
}

// Take module page

type takeModuleRequest struct {
	courseId int
	moduleId int
	blockIdx int
}

func parseTakeModuleRequest(r *http.Request) (takeModuleRequest, error) {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return takeModuleRequest{}, err
	}
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return takeModuleRequest{}, err
	}
	blockIdx, err := strconv.Atoi(r.PathValue("blockIdx"))
	if err != nil {
		return takeModuleRequest{}, err
	}
	return takeModuleRequest{courseId, moduleId, blockIdx}, nil
}

func getModule(ctx HandlerContext, courseId int, moduleId int, moduleVersionId int64) (UiModule, error) {
	module, err := ctx.dbClient.GetModule(courseId, moduleId)
	if err != nil {
		return UiModule{}, err
	}
	moduleVersion, err := ctx.dbClient.GetModuleVersion(moduleVersionId)
	if err != nil {
		return UiModule{}, err
	}
	blockCount, err := ctx.dbClient.GetBlockCount(moduleVersionId)
	if err != nil {
		return UiModule{}, err
	}
	return NewUiModuleStudent(module.CourseId, moduleVersion, blockCount, false, time.Now(), 0), nil
}

func loadUiQuestion(ctx HandlerContext, question db.Question, userId int64) (UiQuestion, error) {
	questionContent, err := ctx.dbClient.GetContent(question.ContentId)
	if err != nil {
		return UiQuestion{}, fmt.Errorf("Error getting question content for question %d: %v", question.Id, err)
	}
	choices, err := ctx.dbClient.GetChoicesForQuestion(question.Id)
	if err != nil {
		return UiQuestion{}, fmt.Errorf("Error getting choices for question %d: %v", question.Id, err)
	}
	choiceContents := make([]db.Content, 0)
	for _, choice := range choices {
		choiceContent, err := ctx.dbClient.GetContent(choice.ContentId)
		if err != nil {
			return UiQuestion{}, fmt.Errorf("Error getting choice content for choice %d: %v", choice.Id, err)
		}
		choiceContents = append(choiceContents, choiceContent)
	}
	explanationContent, err := ctx.dbClient.GetExplanationForQuestion(question.Id)
	if err != nil {
		return UiQuestion{}, fmt.Errorf("Error getting explanation for question %d: %v", question.Id, err)
	}
	choiceId, err := ctx.dbClient.GetAnswer(userId, question.Id)
	if err != nil {
		return UiQuestion{}, fmt.Errorf("Error getting answer for question %d: %v", question.Id, err)
	}

	questionRendered, err := NewUiContentRendered(questionContent)
	if err != nil {
		return UiQuestion{}, fmt.Errorf("Error converting question content for question %d: %v", question.Id, err)
	}
	choicesRendered := make([]UiContent, 0)
	for _, choiceContent := range choiceContents {
		rendered, err := NewUiContentRendered(choiceContent)
		if err != nil {
			return UiQuestion{}, fmt.Errorf("Error converting choice content for question %d: %v", question.Id, err)
		}
		choicesRendered = append(choicesRendered, rendered)
	}
	explanationRendered, err := NewUiContentRendered(explanationContent)
	if err != nil {
		return UiQuestion{}, fmt.Errorf("Error converting explanation content for question %d: %v", question.Id, err)
	}
	var uiQuestion UiQuestion
	if choiceId == -1 {
		uiQuestion = NewUiQuestionTake(question, questionRendered, choices, choicesRendered, explanationRendered)
	} else {
		uiQuestion = NewUiQuestionAnswered(question, questionRendered, choices, choicesRendered, choiceId, explanationRendered)
	}
	return uiQuestion, nil
}

func handleTakeModulePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return err
	}

	prereqs, err := ctx.dbClient.GetPrereqs(moduleId)
	if err != nil {
		return err
	}
	completedAllPrereqs := true
	for _, prereq := range prereqs {
		visit, err := ctx.dbClient.GetVisit(user.Id, prereq.PrereqModuleId)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == sql.ErrNoRows {
			completedAllPrereqs = false
			break
		}
		blockCount, err := ctx.dbClient.GetBlockCount(visit.ModuleVersionId)
		if visit.BlockIndex != blockCount {
			completedAllPrereqs = false
			break
		}
	}
	if !completedAllPrereqs {
		return fmt.Errorf("Cannot take module %d because prereqs are not completed", moduleId)
	}

	visit, err := ctx.dbClient.GetVisit(user.Id, moduleId)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("Error getting visit for module %d: %v", moduleId, err)
	}
	if err == sql.ErrNoRows {
		visit, err = ctx.dbClient.CreateVisit(user.Id, moduleId)
		if err != nil {
			return fmt.Errorf("Error creating visit for module %d: %v", moduleId, err)
		}
	}
	module, err := getModule(ctx, courseId, moduleId, visit.ModuleVersionId)
	if err != nil {
		return fmt.Errorf("Error getting module %d: %v", moduleId, err)
	}
	if visit.BlockIndex > module.BlockCount {
		return fmt.Errorf("Block index %d is out of bounds (>=%d) for module %d", visit.BlockIndex, module.BlockCount, moduleId)
	}
	nBlocks := min(visit.BlockIndex+1, module.BlockCount)
	uiBlocks := make([]UiBlock, 0)
	for blockIdx := 0; blockIdx < nBlocks; blockIdx++ {
		block, err := ctx.dbClient.GetBlock(visit.ModuleVersionId, blockIdx)
		if err != nil {
			return fmt.Errorf("Error getting block %d for module %d: %v", blockIdx, visit.ModuleVersionId, err)
		}

		var uiBlock UiBlock
		switch block.BlockType {
		case db.KnowledgePointBlockType:
			knowledgePoint, err := ctx.dbClient.GetKnowledgePointFromBlock(block.Id)
			if err != nil {
				return fmt.Errorf("Error getting knowledge point for block %d: %v", block.Id, err)
			}

			questionOrders, err := ctx.dbClient.GetQuestionOrders(visit.Id, knowledgePoint.Id)
			if err != nil {
				return fmt.Errorf("Error getting question orders for visit %d and knowledge point %d: %v", visit.Id, knowledgePoint.Id, err)
			}
			var question db.Question
			if len(questionOrders) > 0 {
				// If we've already seen this block, get the latest question order
				// TODO: handle multiple question orders
				questionOrder := questionOrders[len(questionOrders)-1]
				question, err = ctx.dbClient.GetQuestion(questionOrder.QuestionId)
				if err != nil {
					return fmt.Errorf("Error getting question %d: %v", questionOrder.QuestionId, err)
				}
			} else {
				if blockIdx != nBlocks-1 {
					return fmt.Errorf("No question order found for block %d, but not at the last block", block.Id)
				}
				// Since this is the first time we're seeing this block,
				// get a random question and mark the question order
				questions, err := ctx.dbClient.GetLatestQuestionsForKnowledgePoint(knowledgePoint.Id)
				if err != nil {
					return fmt.Errorf("Error getting question for knowledge point %d: %v", knowledgePoint.Id, err)
				}
				questionIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(questions))))
				if err != nil {
					return fmt.Errorf("Error getting random question index: %v", err)
				}
				question = questions[questionIdx.Int64()]
				_, err = ctx.dbClient.InsertQuestionOrder(visit.Id, knowledgePoint.Id, int64(question.Id), 0)
				if err != nil {
					return fmt.Errorf("Error inserting question order for visit %d and knowledge point %d: %v", visit.Id, knowledgePoint.Id, err)
				}
			}

			uiQuestion, err := loadUiQuestion(ctx, question, user.Id)
			if err != nil {
				return fmt.Errorf("Error loading question for block %d: %v", block.Id, err)
			}
			uiBlock = NewUiBlockQuestion(uiQuestion, blockIdx)
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
		default:
			return fmt.Errorf("Unknown block type %s", block.BlockType)
		}

		uiBlocks = append(uiBlocks, uiBlock)
	}
	uiModule := UiTakeModulePage{
		Module:     module,
		Blocks:     uiBlocks,
		VisitIndex: visit.BlockIndex,
		Preview:    false,
	}
	return ctx.renderer.RenderTakeModulePage(w, uiModule)
}

func validateTakeModuleBlockIdx(blockIdx int, module UiModule, visit db.Visit) error {
	if blockIdx >= module.BlockCount {
		return fmt.Errorf("Block index %d is out of bounds (>=%d) for module %d", blockIdx, module.BlockCount, module.Id)
	}
	if blockIdx > visit.BlockIndex+1 {
		return fmt.Errorf("Block index %d is ahead of visit block index %d for module %d", blockIdx, visit.BlockIndex, module.Id)
	}
	return nil
}

func handleTakeModule(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	req, err := parseTakeModuleRequest(r)
	if err != nil {
		return err
	}
	visit, err := ctx.dbClient.GetVisit(user.Id, req.moduleId)
	if err != nil {
		return err
	}
	module, err := getModule(ctx, req.courseId, req.moduleId, visit.ModuleVersionId)
	if err != nil {
		return err
	}
	err = validateTakeModuleBlockIdx(req.blockIdx, module, visit)
	if err != nil {
		return err
	}

	block, err := ctx.dbClient.GetBlock(visit.ModuleVersionId, req.blockIdx)
	if err != nil {
		return fmt.Errorf("Error getting block %d for module %d: %v", req.blockIdx, visit.ModuleVersionId, err)
	}

	var uiBlock UiBlock
	switch block.BlockType {
	case db.KnowledgePointBlockType:
		knowledgePoint, err := ctx.dbClient.GetKnowledgePointFromBlock(block.Id)
		if err != nil {
			return fmt.Errorf("Error getting knowledge point for block %d: %v", block.Id, err)
		}
		// Since this is the first time we're seeing this block,
		// get a random question and mark the question order
		questions, err := ctx.dbClient.GetLatestQuestionsForKnowledgePoint(knowledgePoint.Id)
		if err != nil {
			return fmt.Errorf("Error getting question for knowledge point %d: %v", knowledgePoint.Id, err)
		}
		questionIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(questions))))
		if err != nil {
			return fmt.Errorf("Error getting random question index: %v", err)
		}
		question := questions[questionIdx.Int64()]
		uiQuestion, err := loadUiQuestion(ctx, question, user.Id)
		if err != nil {
			return fmt.Errorf("Error loading question for block %d: %v", block.Id, err)
		}
		uiBlock = NewUiBlockQuestion(uiQuestion, req.blockIdx)

		_, err = ctx.dbClient.InsertQuestionOrder(visit.Id, knowledgePoint.Id, int64(question.Id), 0)
		if err != nil {
			return fmt.Errorf("Error inserting question order for visit %d and knowledge point %d: %v", visit.Id, knowledgePoint.Id, err)
		}
	case db.ContentBlockType:
		content, err := ctx.dbClient.GetContentFromBlock(block.Id)
		if err != nil {
			return fmt.Errorf("Error getting content for block %d: %v", block.Id, err)
		}
		rendered, err := NewUiContentRendered(content)
		if err != nil {
			return fmt.Errorf("Error converting content for block %d: %v", block.Id, err)
		}
		uiBlock = NewUiBlockContent(rendered, req.blockIdx)
	default:
		return fmt.Errorf("Unknown block type %s", block.BlockType)
	}

	err = ctx.dbClient.UpdateVisit(user.Id, visit.ModuleVersionId, req.blockIdx)
	if err != nil {
		return err
	}
	uiTakeModule := UiTakeModule{
		Module:     module,
		Block:      uiBlock,
		VisitIndex: req.blockIdx,
	}
	return ctx.renderer.RenderTakeModule(w, uiTakeModule)
}

func handleAnswerQuestion(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	req, err := parseTakeModuleRequest(r)
	if err != nil {
		return err
	}
	visit, err := ctx.dbClient.GetVisit(user.Id, req.moduleId)
	if err != nil {
		return err
	}
	module, err := getModule(ctx, req.courseId, req.moduleId, visit.ModuleVersionId)
	if err != nil {
		return err
	}
	err = validateTakeModuleBlockIdx(req.blockIdx, module, visit)
	if err != nil {
		return err
	}
	block, err := ctx.dbClient.GetBlock(visit.ModuleVersionId, req.blockIdx)
	if err != nil {
		return fmt.Errorf("Error getting block %d for module %d: %v", req.blockIdx, visit.ModuleVersionId, err)
	}
	if block.BlockType != db.KnowledgePointBlockType {
		return fmt.Errorf("Tried to submit answer, but block at index %d for module %d is not a knowledge point block", req.blockIdx, req.moduleId)
	}

	knowledgePoint, err := ctx.dbClient.GetKnowledgePointFromBlock(block.Id)
	if err != nil {
		return fmt.Errorf("Error getting knowledge point for block %d: %v", block.Id, err)
	}
	// Get the latest question order
	questionOrders, err := ctx.dbClient.GetQuestionOrders(visit.Id, knowledgePoint.Id)
	if err != nil {
		return fmt.Errorf("Error getting question orders for visit %d and knowledge point %d: %v", visit.Id, knowledgePoint.Id, err)
	}
	if len(questionOrders) == 0 {
		return fmt.Errorf("No question orders found for visit %d and knowledge point %d", visit.Id, knowledgePoint.Id)
	}
	questionOrder := questionOrders[len(questionOrders)-1]
	question, err := ctx.dbClient.GetQuestion(questionOrder.QuestionId)
	if err != nil {
		return fmt.Errorf("Error getting question %d: %v", questionOrder.QuestionId, err)
	}

	uiQuestion, err := loadUiQuestion(ctx, question, user.Id)
	if err != nil {
		return fmt.Errorf("Error loading question for block %d: %v", block.Id, err)
	}
	uiBlock := NewUiBlockQuestion(uiQuestion, req.blockIdx)

	uiTakeModule := UiTakeModule{
		Module:     module,
		Block:      uiBlock,
		VisitIndex: req.blockIdx,
	}
	err = r.ParseForm()
	if err != nil {
		return err
	}
	choiceId, err := strconv.Atoi(r.Form.Get("choice"))
	if err != nil {
		return err
	}
	err = ctx.dbClient.StoreAnswer(user.Id, uiTakeModule.Block.Question.Id, choiceId)
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

func handleCompleteModule(w http.ResponseWriter, r *http.Request, ctx HandlerContext, user db.User) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return err
	}
	visit, err := ctx.dbClient.GetVisit(user.Id, moduleId)
	if err != nil {
		return err
	}
	blockCount, err := ctx.dbClient.GetBlockCount(visit.ModuleVersionId)
	if err != nil {
		return err
	}
	if visit.BlockIndex < blockCount-1 {
		return fmt.Errorf("Tried to complete module %d, but only at block index %d", moduleId, visit.BlockIndex)
	}
	if visit.BlockIndex == blockCount {
		// Already completed, skip to redirect
		w.Header().Add("HX-Redirect", fmt.Sprintf("/student/course/%d", courseId))
		return nil
	}

	// Calculate points to award
	blocks, err := ctx.dbClient.GetBlocks(visit.ModuleVersionId)
	if err != nil {
		return err
	}
	correctAnswers := 0
	questionCount := 0
	for _, block := range blocks {
		if block.BlockType == db.KnowledgePointBlockType {
			questionCount += 1
			knowledgePoint, err := ctx.dbClient.GetKnowledgePointFromBlock(block.Id)
			if err != nil {
				return err
			}
			// Figure out getting question for when student has answered
			questions, err := ctx.dbClient.GetQuestionsForKnowledgePoint(knowledgePoint.Id)
			if err != nil {
				return err
			}
			// TODO: handle multiple questions
			question := questions[0]
			choiceId, err := ctx.dbClient.GetAnswer(user.Id, question.Id)
			if err != nil {
				return err
			}
			choice, err := ctx.dbClient.GetChoice(choiceId)
			if err != nil {
				return err
			}
			if choice.Correct {
				correctAnswers += 1
			}
		}
	}
	pointCount := blockCount
	if questionCount > 0 && correctAnswers == 0 {
		// No points if all questions are wrong
		pointCount = 0
	} else if correctAnswers == questionCount {
		// Bonus points for perfect score
		pointCount += pointCount / 4
	} else if correctAnswers == questionCount-1 {
		// No penalty for one mistake
	} else {
		// Get points proportional to correct answers
		pointCount = pointCount * correctAnswers / questionCount
	}

	tx, err := ctx.dbClient.Begin()
	defer tx.Rollback()
	err = db.UpdateVisit(tx, user.Id, visit.ModuleVersionId, blockCount)
	if err != nil {
		return err
	}
	_, err = db.InsertPoint(tx, user.Id, moduleId, pointCount)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	w.Header().Add("HX-Redirect", fmt.Sprintf("/student/course/%d", courseId))
	return nil
}
