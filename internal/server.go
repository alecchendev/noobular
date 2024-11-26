package internal

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/yuin/goldmark"
)

func NewServer(dbClient *DbClient, port int) *http.Server {
	router := initRouter(dbClient)
	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}
}

func initRouter(dbClient *DbClient) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/course/create", NewHandlerMap(dbClient).Get(handleCreateCoursePage).Post(handleCreateCourse))
	mux.Handle("/course/{courseId}/edit", NewHandlerMap(dbClient).Get(handleEditCoursePage).Put(handleEditCourse))
	mux.Handle("/course/{courseId}", NewHandlerMap(dbClient).Delete(handleDeleteCourse))
	mux.Handle("/ui/{questionIdx}/choice", NewHandlerMap(dbClient).Get(handleAddChoice))
	mux.Handle("/ui/{element}", NewHandlerMap(dbClient).Get(handleAddElement).Delete(handleDeleteElement))
	// This is kinda a weird place to put the deleteModuleHandler because it's on a different page
	// (the edit course page) but it's fine for now.
	mux.Handle("/course/{courseId}/module/{moduleId}/edit", NewHandlerMap(dbClient).Get(handleEditModulePage).Put(handleEditModule).Delete(handleDeleteModule))
	mux.Handle("/course", NewHandlerMap(dbClient).Get(handleCoursesPage))
	mux.Handle("/student/course", NewHandlerMap(dbClient).Get(handleStudentCoursesPage))
	mux.Handle("/student/course/{courseId}/module/{moduleId}/block/{blockIdx}", NewHandlerMap(dbClient).Get(handleTakeModulePage))
	mux.Handle("/student/course/{courseId}/module/{moduleId}/block/{blockIdx}/piece", NewHandlerMap(dbClient).Get(handleTakeModule))
	mux.Handle("/student/course/{courseId}/module/{moduleId}/block/{blockIdx}/answer", NewHandlerMap(dbClient).Post(handleAnswerQuestion))
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.Handle("/style/", http.StripPrefix("/style/", http.FileServer(http.Dir("style"))))
	mux.Handle("/", NewHandlerMap(dbClient).Get(handleHomePage))
	return mux
}

// Things that all handlers should have access to
type HandlerContext struct {
	dbClient *DbClient
	renderer Renderer
}

func NewHandlerContext(dbClient *DbClient, renderer Renderer) HandlerContext {
	return HandlerContext{dbClient, renderer}
}

// Basically an http.Handle but returns an error
type HandlerMapHandler func(http.ResponseWriter, *http.Request, HandlerContext) error

type HandlerMap struct {
	handlers        map[string]HandlerMapHandler
	ctx             HandlerContext
	reloadTemplates bool
}

func (hm HandlerMap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var requestIdOpt uuid.NullUUID
	requestIdOpt.UnmarshalText([]byte(r.Header.Get("X-Request-Id")))
	var requestId uuid.UUID
	if requestIdOpt.Valid {
		requestId = requestIdOpt.UUID
	} else {
		requestId = uuid.New()
	}
	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Println(requestId, r.Method, r.URL.Path, r.Form)
	if hm.reloadTemplates {
		// Reload templates so we don't have to restart the server
		// to see changes
		hm.ctx.renderer.refreshTemplates()
	}
	if handler, ok := hm.handlers[r.Method]; ok {
		err := handler(w, r, hm.ctx)
		if err != nil {
			log.Println(requestId, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	log.Printf("%s Method %s not allowed for path %s", requestId, r.Method, r.URL.Path)
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func NewHandlerMap(dbClient *DbClient) HandlerMap {
	return HandlerMap{
		handlers:        make(map[string]HandlerMapHandler),
		ctx:             NewHandlerContext(dbClient, NewRenderer()),
		reloadTemplates: true,
	}
}

func (hm HandlerMap) Get(handler HandlerMapHandler) HandlerMap {
	hm.handlers["GET"] = handler
	return hm
}

func (hm HandlerMap) Post(handler HandlerMapHandler) HandlerMap {
	hm.handlers["POST"] = handler
	return hm
}

func (hm HandlerMap) Put(handler HandlerMapHandler) HandlerMap {
	hm.handlers["PUT"] = handler
	return hm
}

func (hm HandlerMap) Delete(handler HandlerMapHandler) HandlerMap {
	hm.handlers["DELETE"] = handler
	return hm
}

// Home page

func handleHomePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	if r.URL.Path != "/" {
		// TODO: We should return 404 here
		return fmt.Errorf("Not found")
	}
	return ctx.renderer.RenderHomePage(w)
}

// Courses page

func handleCoursesPage(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	newCourseId, err := strconv.Atoi(r.URL.Query().Get("newCourse"))
	if err != nil {
		newCourseId = -1
	}
	courses, err := ctx.dbClient.GetCourses(false)
	if err != nil {
		return err
	}
	uiCourses := make([]UiCourse, len(courses))
	for i, course := range courses {
		modules, err := ctx.dbClient.GetModules(course.Id, false)
		if err != nil {
			return err
		}
		uiModules := make([]UiModule, len(modules))
		for j, module := range modules {
			uiModules[j] = NewUiModule(module)
		}
		uiCourses[i] = UiCourse{course.Id, course.Title, course.Description, uiModules}
	}
	return ctx.renderer.RenderTeacherCoursePage(w, uiCourses, newCourseId)
}

func handleStudentCoursesPage(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	courses, err := ctx.dbClient.GetCourses(true)
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
			nextUnansweredQuestionIdx, err := ctx.dbClient.GetNextUnansweredQuestionIdx(module.Id)
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

// Delete course (on teacher courses page)

func handleDeleteCourse(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	err = ctx.dbClient.DeleteCourse(courseId)
	if err != nil {
		return err
	}
	return nil
}

// Create course page

func handleCreateCoursePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	return ctx.renderer.RenderCreateCoursePage(w)
}

type createCourseRequest struct {
	title              string
	description        string
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
	moduleTitles := r.Form["module-title[]"]
	moduleDescriptions := r.Form["module-description[]"]
	return createCourseRequest{title, description, moduleTitles, moduleDescriptions}, nil
}

func handleCreateCourse(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	req, err := parseCreateCourseRequest(r)
	if err != nil {
		return err
	}
	course, _, err := ctx.dbClient.CreateCourse(req.title, req.description, req.moduleTitles, req.moduleDescriptions)
	if err != nil {
		return err
	}
	w.Header().Add("HX-Redirect", fmt.Sprintf("/course?newCourse=%d#course-%d", course.Id, course.Id))
	return nil
}

// Edit course page

func handleEditCoursePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	course, err := ctx.dbClient.GetCourse(courseId)
	if err != nil {
		return err
	}
	modules, err := ctx.dbClient.GetModules(courseId, false)
	if err != nil {
		return err
	}
	uiModules := make([]UiModule, len(modules))
	for i, module := range modules {
		uiModules[i] = NewUiModule(module)
	}
	return ctx.renderer.RenderEditCoursePage(w, UiCourse{course.Id, course.Title, course.Description, uiModules})
}

type editCourseRequest struct {
	courseId           int
	title              string
	description        string
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
	return editCourseRequest{courseId, title, description, moduleIds, moduleTitles, moduleDescriptions}, nil
}

func handleEditCourse(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	req, err := parseEditCourseRequest(r)
	if err != nil {
		return err
	}
	_, _, err = ctx.dbClient.EditCourse(req.courseId, req.title, req.description, req.moduleIds, req.moduleTitles, req.moduleDescriptions)
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

func handleDeleteModule(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
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

func handleEditModulePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	// Get courseId and moduleId from "/course/:courseId/module/:moduleId/edit"
	courseId, err := strconv.Atoi(r.PathValue("courseId"))
	if err != nil {
		return err
	}
	moduleId, err := strconv.Atoi(r.PathValue("moduleId"))
	if err != nil {
		return err
	}
	course, err := ctx.dbClient.GetCourse(courseId)
	if err != nil {
		return err
	}
	module, err := ctx.dbClient.GetModule(moduleId)
	if err != nil {
		return err
	}
	blocks, err := ctx.dbClient.GetBlocks(moduleId)
	if err != nil {
		return err
	}
	uiBlocks := make([]UiBlock, len(blocks))
	for _, block := range blocks {
		uiBlock := UiBlock{BlockType: block.BlockType}
		if block.BlockType == ContentBlockType {
			content, err := ctx.dbClient.GetContentFromBlock(block.Id)
			if err != nil {
				return err
			}
			uiBlock.Content = NewUiContent(content)
		} else if block.BlockType == QuestionBlockType {
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
			uiBlock.Question = NewUiQuestion(question, choices, explanation)
		} else {
			return fmt.Errorf("invalid block type: %s", block.BlockType)
		}
		uiBlocks = append(uiBlocks, uiBlock)
	}
	return ctx.renderer.RenderEditModulePage(w, UiEditModule{
		CourseId:    courseId,
		CourseTitle: course.Title,
		ModuleId:    moduleId,
		ModuleTitle: module.Title,
		ModuleDesc:  module.Description,
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

func handleEditModule(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	req, err := parseEditModuleRequest(r)
	if err != nil {
		return err
	}
	err = ctx.dbClient.EditModule(req.moduleId, req.title, req.description, req.blockTypes, req.contents, req.questions, req.choicesByQuestion, req.correctChoiceIdxs, req.explanations)
	if err != nil {
		return err
	}
	return ctx.renderer.RenderModuleEdited(w)
}

// Take module page

func getTakeModule(w http.ResponseWriter, r *http.Request, ctx HandlerContext) (UiTakeModule, error) {
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
	if block.BlockType == QuestionBlockType {
		question, err := ctx.dbClient.GetQuestionFromBlock(block.Id)
		if err != nil {
			return UiTakeModule{}, err
		}
		choiceId, err := ctx.dbClient.GetAnswer(question.Id)
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
			BlockType:       string(QuestionBlockType),
			Content:         template.HTML(""),
			QuestionCount:   blockCount,
			QuestionIndex:   blockIdx,
			ChosenChoiceId:  choiceId,
			CorrectChoiceId: correctChoiceId,
			Question:        NewUiQuestion(question, choices, explanationContent),
			Explanation:     explanation,
		}, nil
	} else if block.BlockType == ContentBlockType {
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
			BlockType:       string(ContentBlockType),
			Content:         template.HTML(buf.String()),
			QuestionCount:   blockCount,
			QuestionIndex:   blockIdx,
			ChosenChoiceId:  -1,
			CorrectChoiceId: -1,
			Question:        EmptyQuestion(),
			Explanation:     template.HTML(""),
		}, nil
	} else {
		return UiTakeModule{}, fmt.Errorf("Unknown block type %s", block.BlockType)
	}
}

func handleTakeModulePage(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	module, err := getTakeModule(w, r, ctx)
	if err != nil {
		return err
	}
	return ctx.renderer.RenderTakeModulePage(w, module)
}

func handleTakeModule(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	module, err := getTakeModule(w, r, ctx)
	if err != nil {
		return err
	}
	return ctx.renderer.RenderTakeModule(w, module)
}

func handleAnswerQuestion(w http.ResponseWriter, r *http.Request, ctx HandlerContext) error {
	uiTakeModule, err := getTakeModule(w, r, ctx)
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
	err = ctx.dbClient.StoreAnswer(uiTakeModule.Question.Id, choiceId)
	if err != nil {
		return err
	}

	return ctx.renderer.RenderQuestionSubmitted(w, UiSubmittedAnswer{
		Module:          uiTakeModule.Module,
		QuestionCount:   uiTakeModule.QuestionCount,
		QuestionIndex:   uiTakeModule.QuestionIndex,
		ChosenChoiceId:  choiceId,
		CorrectChoiceId: uiTakeModule.CorrectChoiceId,
		Question:        uiTakeModule.Question,
		Explanation:     uiTakeModule.Explanation,
	})
}

// Render individual module (for switching)
