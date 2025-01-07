package internal

import (
	"bytes"
	"fmt"
	"html/template"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/graemephi/goldmark-qjs-katex"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"

	"noobular/internal/db"
)

type Renderer struct {
	projectRootDir string
	templates      map[string]*template.Template
}

func NewRenderer(projectRootDir string) Renderer {
	return Renderer{
		projectRootDir: projectRootDir,
		templates:      initTemplates(projectRootDir),
	}
}

func (r *Renderer) refreshTemplates() {
	r.templates = initTemplates(r.projectRootDir)
}

func initTemplates(projectRootDir string) map[string]*template.Template {
	funcMap := template.FuncMap{
		"TitleCase": func(s string) string {
			return strings.Title(strings.ToLower(s))
		},
		"EmptyModule": func() UiModule {
			return EmptyModule()
		},
		"EmptyQuestion": func() UiQuestion {
			return EmptyQuestion()
		},
		"EmptyChoice": func(questionIdx int) UiChoice {
			return EmptyChoice(questionIdx)
		},
		"NumRange": func(n int) []int {
			nums := make([]int, n)
			for i := 0; i < n; i++ {
				nums[i] = i
			}
			return nums
		},
		"Increment": func(i int) int {
			return i + 1
		},
	}
	filePaths := map[string][]string{
		"index.html":         {"page.html", "index.html"},
		"signup.html":        {"page.html", "signup.html"},
		"student.html":       {"page.html", "student.html"},
		"courses.html":       {"page.html", "courses.html"},
		"create_course.html": {"page.html", "create_course.html",
				       "add_element.html",
				       "created_course_response.html",
				       "edited_course_response.html"},
		"edit_module.html":   {"page.html", "edit_module.html",
				       "add_element.html",
				       "edited_module_response.html"},
		"prereq.html":        {"page.html", "prereq.html"},
		"knowledge_point.html": {"page.html", "knowledge_point.html"},
		"take_module.html":   {"page.html", "take_module.html"},
		"add_element.html":   {"add_element.html"},
		"export_module.html": {"export_module.html"},
	}
	templates := make(map[string]*template.Template)
	for name, paths := range filePaths {
		files := make([]string, len(paths))
		for i, path := range paths {
			files[i] = projectRootDir + "/template/" + path
		}
		templates[name] = template.Must(template.New("").Funcs(funcMap).ParseFiles(files...))
	}
	return templates
}

type PageArgs struct {
	ShowNav     bool
	LoggedIn    bool
	ContentArgs interface{}
}

func NewPageArgs(showNav, loggedIn bool, contentArgs interface{}) PageArgs {
	return PageArgs{showNav, loggedIn, contentArgs}
}

// Home page - optional login
// Browse page - optional login
// Sign in/up page - not logged in
// Teacher page - logged in
// Student page - logged in
// Take module page - optional login

// Basic welcome page when a user is not logged in.
func (r *Renderer) RenderHomePage(w http.ResponseWriter, loggedIn bool) error {
	return r.templates["index.html"].ExecuteTemplate(w, "page.html", NewPageArgs(true, loggedIn, nil))
}

type SignupPageArgs struct {
	Signin bool
}

func (r *Renderer) RenderSignupPage(w http.ResponseWriter) error {
	return r.templates["signup.html"].ExecuteTemplate(w, "page.html", NewPageArgs(true, false, SignupPageArgs{false}))
}

func (r *Renderer) RenderSigninPage(w http.ResponseWriter) error {
	return r.templates["signup.html"].ExecuteTemplate(w, "page.html", NewPageArgs(true, false, SignupPageArgs{true}))
}

func (r *Renderer) RenderBrowsePage(w http.ResponseWriter, courses []UiCourse, loggedIn bool) error {
	return r.templates["courses.html"].ExecuteTemplate(w, "page.html", NewPageArgs(true, loggedIn, CoursePageArgs{0, false, loggedIn, courses}))
}

// Course struct for feeding into a template to be rendered
type UiCourse struct {
	Id          int
	Title       string
	Description string
	Public      bool
	Modules     []UiModule
	Enrolled    bool
}

func NewUiCourse(c db.Course, modules []UiModule) UiCourse {
	return NewUiCourseEnrolled(c, modules, false)
}

func NewUiCourseEnrolled(c db.Course, modules []UiModule, enrolled bool) UiCourse {
	return UiCourse{c.Id, c.Title, c.Description, c.Public, modules, enrolled}
}

func EmptyCourse() UiCourse {
	return UiCourse{-1, "", "", true, []UiModule{}, false}
}

func (c UiCourse) HasStudent() bool {
	return false
}

// This is a course that has at least one student.
// This means we can no longer take this course private.
type UiFixedPublicCourse UiCourse

func (c UiFixedPublicCourse) HasStudent() bool {
	return true
}

type UiModule struct {
	Id          int
	CourseId    int
	Title       string
	Description string
	BlockCount  int
	Completed   bool
	CompletedAt time.Time
	Points      int
}

func NewUiModuleTeacher(courseId int, version db.ModuleVersion) UiModule {
	return UiModule{version.ModuleId, courseId, version.Title, version.Description, 0, false, time.Now(), 0}
}

func NewUiModuleStudent(courseId int, version db.ModuleVersion, blockCount int, completed bool, completedAt time.Time, points int) UiModule {
	return UiModule{version.ModuleId, courseId, version.Title, version.Description, blockCount, completed, completedAt, points}
}

func EmptyModule() UiModule {
	return UiModule{-1, -1, "", "", 0, false, time.Now(), 0}
}

func (m UiModule) ElementType() string {
	return "module"
}

func (m UiModule) ElementText() string {
	return m.Title
}

func (m UiModule) IsEmpty() bool {
	return m.Id == -1
}

type UiBlock struct {
	BlockType  db.BlockType
	BlockIndex int
	Content    UiContent
	Question   UiQuestion
}

func NewUiBlockQuestion(question UiQuestion, idx int) UiBlock {
	return UiBlock{db.KnowledgePointBlockType, idx, EmptyContent(), question}
}

func NewUiBlockContent(content UiContent, idx int) UiBlock {
	return UiBlock{db.ContentBlockType, idx, content, EmptyQuestion()}
}

type UiQuestion struct {
	Id int
	// This is a random integer created to differentiate questions in the UI.
	Idx         int
	Content     UiContent
	Choices     []UiChoice
	Explanation UiContent
}

func NewUiQuestionEdit(q db.Question, content db.Content, choices []db.Choice, choiceContents []db.Content, explanation db.Content) UiQuestion {
	questionIdx := rand.Int()
	uiChoices := make([]UiChoice, 0)
	for i, choice := range choices {
		uiChoices = append(uiChoices, NewUiChoice(questionIdx, choice, NewUiContent(choiceContents[i])))
	}
	return UiQuestion{q.Id, questionIdx, NewUiContent(content), uiChoices, NewUiContent(explanation)}
}

func NewUiQuestionTake(q db.Question, content UiContent, choices []db.Choice, choiceContents []UiContent, explanation UiContent) UiQuestion {
	questionIdx := rand.Int()
	uiChoices := make([]UiChoice, len(choices))
	for i, choice := range choices {
		uiChoices[i] = NewUiChoice(questionIdx, choice, choiceContents[i])
	}
	return UiQuestion{q.Id, questionIdx, content, uiChoices, explanation}
}

func NewUiQuestionAnswered(q db.Question, content UiContent, choices []db.Choice, choiceContents []UiContent, chosenChoiceId int, explanation UiContent) UiQuestion {
	questionIdx := rand.Int()
	uiChoices := make([]UiChoice, len(choices))
	for i, choice := range choices {
		if choice.Id == chosenChoiceId {
			uiChoices[i] = NewUiChoiceChosen(questionIdx, choice, choiceContents[i])
		} else {
			uiChoices[i] = NewUiChoice(questionIdx, choice, choiceContents[i])
		}
	}
	return UiQuestion{q.Id, questionIdx, content, uiChoices, explanation}
}

func (q UiQuestion) Answered() bool {
	for _, choice := range q.Choices {
		if choice.Chosen {
			return true
		}
	}
	return false
}

func (q UiQuestion) AnsweredCorrectly() bool {
	for _, choice := range q.Choices {
		if choice.Chosen && choice.IsCorrect {
			return true
		}
	}
	return false
}

func EmptyQuestion() UiQuestion {
	return UiQuestion{-1, rand.Int(), EmptyContent(), []UiChoice{}, EmptyContent()}
}

func (q UiQuestion) ElementType() string {
	return "question"
}

func (q UiQuestion) ElementText() string {
	return q.Content.Content
}

func (q UiQuestion) IsEmpty() bool {
	return q.Id == -1
}

type UiChoice struct {
	Id int
	// This is a random integer created to differentiate questions in the UI.
	QuestionIdx int
	/// A random idx just to differentiate choices in the UI
	/// so that label elements can be associated with certain choices.
	Idx       int
	Content   UiContent
	Chosen    bool
	IsCorrect bool
}

func NewUiChoice(questionIdx int, c db.Choice, content UiContent) UiChoice {
	return UiChoice{c.Id, questionIdx, rand.Int(), content, false, c.Correct}
}

func NewUiChoiceChosen(questionIdx int, c db.Choice, content UiContent) UiChoice {
	return UiChoice{c.Id, questionIdx, rand.Int(), content, true, c.Correct}
}

func EmptyChoice(questionIdx int) UiChoice {
	return UiChoice{-1, questionIdx, rand.Int(), UiContent{}, false, false}
}

func (c UiChoice) ElementType() string {
	return "choice"
}

func (c UiChoice) ElementText() string {
	return c.Content.Content
}

func (c UiChoice) IsEmpty() bool {
	return c.Id == -1
}

type CoursePageArgs struct {
	NewCourseId int
	Editor      bool
	LoggedIn    bool
	Courses     []UiCourse
}

type StudentPageArgs struct {
	Username string
	Courses  []UiCourse
}

func (a StudentPageArgs) HasCourse() bool {
	return false
}

type StudentCoursePageArgs struct {
	Username    string
	Course      UiCourse
	TotalPoints int
}

func (a StudentCoursePageArgs) HasCourse() bool {
	return true
}

func (a StudentCoursePageArgs) HasNextModules() bool {
	uncompletedModules := 0
	for _, module := range a.Course.Modules {
		if !module.Completed {
			uncompletedModules++
		}
	}
	return uncompletedModules > 0
}

func (a StudentCoursePageArgs) HasCompletedModules() bool {
	completedModules := 0
	for _, module := range a.Course.Modules {
		if module.Completed {
			completedModules++
		}
	}
	return completedModules > 0
}

func (r *Renderer) RenderStudentPage(w http.ResponseWriter, args StudentPageArgs) error {
	return r.templates["student.html"].ExecuteTemplate(w, "page.html", NewPageArgs(true, true, args))
}

func (r *Renderer) RenderStudentCoursePage(w http.ResponseWriter, args StudentCoursePageArgs) error {
	return r.templates["student.html"].ExecuteTemplate(w, "page.html", NewPageArgs(true, true, args))
}

func (r *Renderer) RenderTeacherCoursePage(w http.ResponseWriter, courses []UiCourse, newCourseId int) error {
	return r.templates["courses.html"].ExecuteTemplate(w, "page.html", NewPageArgs(true, true, CoursePageArgs{newCourseId, true, true, courses}))
}

func (r *Renderer) RenderCreateCoursePage(w http.ResponseWriter) error {
	return r.templates["create_course.html"].ExecuteTemplate(w, "page.html", NewPageArgs(true, true, EmptyCourse()))
}

func (r *Renderer) RenderCourseCreated(w http.ResponseWriter) error {
	return r.templates["create_course.html"].ExecuteTemplate(w, "created_course_response.html", nil)
}

func (r *Renderer) RenderEditCoursePage(w http.ResponseWriter, course UiCourse, publicFixed bool) error {
	var uiCourse interface{} = course
	if publicFixed {
		uiCourse = UiFixedPublicCourse(course)
	}
	return r.templates["create_course.html"].ExecuteTemplate(w, "page.html", NewPageArgs(true, true, uiCourse))
}

func (r *Renderer) RenderCourseEdited(w http.ResponseWriter) error {
	return r.templates["create_course.html"].ExecuteTemplate(w, "edited_course_response.html", nil)
}

func (r *Renderer) RenderNewModule(w http.ResponseWriter, module UiModule) error {
	return r.templates["add_element.html"].ExecuteTemplate(w, "add_element.html", module)
}

func (r *Renderer) RenderNewQuestion(w http.ResponseWriter, question UiQuestion) error {
	return r.templates["add_element.html"].ExecuteTemplate(w, "add_element.html", question)
}

type UiContent struct {
	Id int
	// This is a random integer created to differentiate questions in the UI.
	Idx         int
	Content     string
	ContentTmpl template.HTML
}

func NewUiContent(content db.Content) UiContent {
	return UiContent{content.Id, rand.Int(), content.Content, template.HTML("")}
}

func newMd() goldmark.Markdown {
	// For some of these, I should consider how rendering
	// these fits into the protocol. For example, I want
	// to allow people to include custom interactive diagrams
	// via iframes, but the requirement to support this makes
	// the protocol more web-centric.
	return goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // For iframes, etc.
		),
		goldmark.WithExtensions(
			&qjskatex.Extension{},
			extension.Table,
		),
	)
}

func NewUiContentRendered(content db.Content) (UiContent, error) {
	var buf bytes.Buffer
	if err := newMd().Convert([]byte(content.Content), &buf); err != nil {
		return UiContent{}, fmt.Errorf("Error converting content: %v", err)
	}
	return UiContent{content.Id, rand.Int(), content.Content, template.HTML(buf.String())}, nil
}

func EmptyContent() UiContent {
	return UiContent{-1, rand.Int(), "", template.HTML("")}
}

func (c UiContent) ElementType() string {
	return "content"
}

func (c UiContent) ElementText() string {
	return c.Content
}

func (c UiContent) IsEmpty() bool {
	return c.Id == -1
}

func (r *Renderer) RenderNewContent(w http.ResponseWriter, content UiContent) error {
	return r.templates["add_element.html"].ExecuteTemplate(w, "add_element.html", content)
}

func (r *Renderer) RenderNewChoice(w http.ResponseWriter, choice UiChoice) error {
	return r.templates["add_element.html"].ExecuteTemplate(w, "add_element.html", choice)
}

type UiEditModule struct {
	CourseId    int
	CourseTitle string
	ModuleId    int
	ModuleTitle string
	ModuleDesc  string
	Blocks      []UiBlock
}

func (r *Renderer) RenderEditModulePage(w http.ResponseWriter, module UiEditModule) error {
	return r.templates["edit_module.html"].ExecuteTemplate(w, "page.html", NewPageArgs(true, true, module))
}

func (r *Renderer) RenderModuleEdited(w http.ResponseWriter) error {
	return r.templates["edit_module.html"].ExecuteTemplate(w, "edited_module_response.html", nil)
}

type UiPrereqPageArgs struct {
	Course     UiCourse
	PrereqForm UiPrereqForm
}

type UiPrereq struct {
	Module UiModule
	Prereq bool
}

func NewUiPrereq(m UiModule, prereq bool) UiPrereq {
	return UiPrereq{m, prereq}
}

func (r *Renderer) RenderPrereqPage(w http.ResponseWriter, pageArgs UiPrereqPageArgs) error {
	return r.templates["prereq.html"].ExecuteTemplate(w, "page.html", NewPageArgs(true, true, pageArgs))
}

type UiPrereqForm struct {
	Module  UiModule
	Prereqs []UiPrereq
}

func (r *Renderer) RenderPrereqForm(w http.ResponseWriter, prereqForm UiPrereqForm) error {
	return r.templates["prereq.html"].ExecuteTemplate(w, "prereq_form", prereqForm)
}

func (r *Renderer) RenderPrereqEditedResponse(w http.ResponseWriter, module UiModule) error {
	return r.templates["prereq.html"].ExecuteTemplate(w, "edit_prereq_response", module)
}

type UiKnowledgePointPageArgs struct {
	CourseId int64
	CourseTitle string
	KnowledgePoints []UiKnowledgePoint
}

type UiKnowledgePoint struct {
	Id int64
	Name string
}

func NewUiKnowledgePoint(k db.KnowledgePoint) UiKnowledgePoint {
	return UiKnowledgePoint{k.Id, k.Name}
}

func (r *Renderer) RenderKnowledgePointPage(w http.ResponseWriter, pageArgs UiKnowledgePointPageArgs) error {
	return r.templates["knowledge_point.html"].ExecuteTemplate(w, "page.html", NewPageArgs(true, true, pageArgs))
}

type UiTakeModulePage struct {
	Module     UiModule
	Blocks     []UiBlock
	VisitIndex int
	Preview    bool
}

func (u UiTakeModulePage) IsPage() bool {
	return true
}

func (u UiTakeModulePage) ModuleBlock(index int) UiTakeModule {
	return UiTakeModule{
		Module:     u.Module,
		Block:      u.Blocks[index],
		VisitIndex: u.VisitIndex,
		Preview:    u.Preview,
	}
}

func (r *Renderer) RenderTakeModulePage(w http.ResponseWriter, module UiTakeModulePage) error {
	return r.templates["take_module.html"].ExecuteTemplate(w, "page.html", NewPageArgs(false, true, module))
}

type UiTakeModule struct {
	Module     UiModule
	Block      UiBlock
	VisitIndex int
	Preview    bool
}

func (u UiTakeModule) IsPage() bool {
	return false
}

func (u UiTakeModule) ShowNextButton() bool {
	return u.Block.BlockIndex == u.VisitIndex ||
		(u.VisitIndex == u.Module.BlockCount && u.Block.BlockIndex == u.Module.BlockCount-1)
}

// Renders just the content, i.e. the header + content, not the full page.
func (r *Renderer) RenderTakeModule(w http.ResponseWriter, module UiTakeModule) error {
	return r.templates["take_module.html"].ExecuteTemplate(w, "content_inner", module)
}

func (r *Renderer) RenderQuestionSubmitted(w http.ResponseWriter, module UiTakeModule) error {
	return r.templates["take_module.html"].ExecuteTemplate(w, "question_submitted", module)
}

func (r *Renderer) RenderExportedModule(w http.ResponseWriter, text string) error {
	return r.templates["export_module.html"].ExecuteTemplate(w, "export_module.html", template.HTML(text)) // Use template.HTML to prevent escaping
}
