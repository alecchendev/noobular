package internal

import (
	"html/template"
	"math/rand/v2"
	"net/http"
	"strings"

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
		"create_course.html": {"page.html", "create_course.html", "add_element.html", "created_course_response.html", "edited_course_response.html"},
		"edit_module.html":   {"page.html", "edit_module.html", "add_element.html", "edited_module_response.html"},
		"take_module.html":   {"page.html", "take_module.html"},
		"add_element.html":   {"add_element.html"},
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
	Modules     []UiModule
	Enrolled    bool
}

func NewUiCourse(c db.Course, modules []UiModule) UiCourse {
	return NewUiCourseEnrolled(c, modules, false)
}

func NewUiCourseEnrolled(c db.Course, modules []UiModule, enrolled bool) UiCourse {
	return UiCourse{c.Id, c.Title, c.Description, modules, enrolled}
}

func EmptyCourse() UiCourse {
	return UiCourse{-1, "", "", []UiModule{}, false}
}

type UiModule struct {
	Id          int
	CourseId    int
	Title       string
	Description string
	BlockCount  int
}

func NewUiModuleTeacher(courseId int, version db.ModuleVersion) UiModule {
	return UiModule{version.ModuleId, courseId, version.Title, version.Description, 0}
}

func NewUiModuleStudent(courseId int, version db.ModuleVersion, blockCount int) UiModule {
	return UiModule{version.ModuleId, courseId, version.Title, version.Description, blockCount}
}

func EmptyModule() UiModule {
	return UiModule{-1, -1, "", "", 0}
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
	return UiBlock{db.QuestionBlockType, idx, EmptyContent(), question}
}

func NewUiBlockContent(content UiContent, idx int) UiBlock {
	return UiBlock{db.ContentBlockType, idx, content, EmptyQuestion()}
}

type UiQuestion struct {
	Id int
	// This is a random integer created to differentiate questions in the UI.
	Idx          int
	QuestionText string
	Choices      []UiChoice
	Explanation  UiContent
}

func NewUiQuestionEdit(q db.Question, choices []db.Choice, explanation db.Content) UiQuestion {
	questionIdx := rand.Int()
	uiChoices := make([]UiChoice, len(choices))
	for i, choice := range choices {
		uiChoices[i] = NewUiChoice(questionIdx, choice)
	}
	return UiQuestion{q.Id, questionIdx, q.QuestionText, uiChoices, NewUiContent(explanation)}
}

func NewUiQuestionTake(q db.Question, choices []db.Choice, explanation UiContent) UiQuestion {
	questionIdx := rand.Int()
	uiChoices := make([]UiChoice, len(choices))
	for i, choice := range choices {
		uiChoices[i] = NewUiChoice(questionIdx, choice)
	}
	return UiQuestion{q.Id, questionIdx, q.QuestionText, uiChoices, explanation}
}

func NewUiQuestionAnswered(q db.Question, choices []db.Choice, chosenChoiceId int, explanation UiContent) UiQuestion {
	questionIdx := rand.Int()
	uiChoices := make([]UiChoice, len(choices))
	for i, choice := range choices {
		if choice.Id == chosenChoiceId {
			uiChoices[i] = NewUiChoiceChosen(questionIdx, choice)
		} else {
			uiChoices[i] = NewUiChoice(questionIdx, choice)
		}
	}
	return UiQuestion{q.Id, questionIdx, q.QuestionText, uiChoices, explanation}
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
	return UiQuestion{-1, rand.Int(), "", []UiChoice{}, EmptyContent()}
}

func (q UiQuestion) ElementType() string {
	return "question"
}

func (q UiQuestion) ElementText() string {
	return q.QuestionText
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
	Idx        int
	ChoiceText string
	Chosen     bool
	IsCorrect  bool
}

func NewUiChoice(questionIdx int, c db.Choice) UiChoice {
	return UiChoice{c.Id, questionIdx, rand.Int(), c.ChoiceText, false, c.Correct}
}

func NewUiChoiceChosen(questionIdx int, c db.Choice) UiChoice {
	return UiChoice{c.Id, questionIdx, rand.Int(), c.ChoiceText, true, c.Correct}
}

func EmptyChoice(questionIdx int) UiChoice {
	return UiChoice{-1, questionIdx, rand.Int(), "", false, false}
}

func (c UiChoice) ElementType() string {
	return "choice"
}

func (c UiChoice) ElementText() string {
	return c.ChoiceText
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

func (r *Renderer) RenderEditCoursePage(w http.ResponseWriter, course UiCourse) error {
	return r.templates["create_course.html"].ExecuteTemplate(w, "page.html", NewPageArgs(true, true, course))
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

func NewUiContentRendered(content db.Content, tmpl template.HTML) UiContent {
	return UiContent{content.Id, rand.Int(), content.Content, tmpl}
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
	return r.templates["take_module.html"].ExecuteTemplate(w, "content", module)
}

func (r *Renderer) RenderQuestionSubmitted(w http.ResponseWriter, module UiTakeModule) error {
	return r.templates["take_module.html"].ExecuteTemplate(w, "question_submitted", module)
}
