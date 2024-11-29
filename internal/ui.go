package internal

import (
	"html/template"
	"math/rand/v2"
	"net/http"
	"strings"
)

type Renderer struct {
	templates map[string]*template.Template
}

func NewRenderer() Renderer {
	return Renderer{
		templates: initTemplates(),
	}
}

func (r *Renderer) refreshTemplates() {
	r.templates = initTemplates()
}

func initTemplates() map[string]*template.Template {
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
	return map[string]*template.Template{
		// Pages
		"index.html":   template.Must(template.ParseFiles("template/page.html", "template/index.html")),
		"signup.html":  template.Must(template.ParseFiles("template/page.html", "template/signup.html")),
		"student.html": template.Must(template.ParseFiles("template/page.html", "template/student.html")),
		"courses.html": template.Must(template.ParseFiles("template/page.html", "template/courses.html")),
		"create_course.html": template.Must(template.New("").Funcs(funcMap).ParseFiles(
			"template/page.html", "template/create_course.html",
			"template/add_element.html", "template/created_course_response.html",
			"template/edited_course_response.html")),
		"edit_module.html": template.Must(template.New("").Funcs(funcMap).ParseFiles(
			"template/page.html", "template/edit_module.html", "template/add_element.html",
			"template/edited_module_response.html")),
		"take_module.html": template.Must(template.New("").Funcs(funcMap).ParseFiles(
			"template/page.html", "template/take_module.html")),
		// Standalone partials
		"add_element.html": template.Must(template.New("").Funcs(funcMap).ParseFiles("template/add_element.html")),
	}
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
}

func EmptyCourse() UiCourse {
	return UiCourse{-1, "", "", []UiModule{}}
}

type UiCourseStudent struct {
	Id          int
	Title       string
	Description string
	Modules     []UiModuleStudent
}

type UiModule struct {
	Id          int
	CourseId    int
	Title       string
	Description string
}

func NewUiModule(m Module) UiModule {
	return UiModule{m.Id, m.CourseId, m.Title, m.Description}
}

func EmptyModule() UiModule {
	return UiModule{-1, -1, "", ""}
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

type UiModuleStudent struct {
	Id                        int
	CourseId                  int
	Title                     string
	Description               string
	QuestionCount             int
	NextUnansweredQuestionIdx int
}

type UiBlock struct {
	BlockType BlockType
	Content   UiContent
	Question  UiQuestion
}

type UiQuestion struct {
	Id int
	// This is a random integer created to differentiate questions in the UI.
	Idx          int
	QuestionText string
	Choices      []UiChoice
	Explanation  string
}

func NewUiQuestion(q Question, choices []Choice, explanation Content) UiQuestion {
	questionIdx := rand.Int()
	uiChoices := make([]UiChoice, len(choices))
	for i, choice := range choices {
		uiChoices[i] = NewUiChoice(questionIdx, choice)
	}
	return UiQuestion{q.Id, questionIdx, q.QuestionText, uiChoices, explanation.Content}
}

func EmptyQuestion() UiQuestion {
	return UiQuestion{-1, rand.Int(), "", []UiChoice{}, ""}
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
	IsCorrect  bool
}

func NewUiChoice(questionIdx int, c Choice) UiChoice {
	return UiChoice{c.Id, questionIdx, rand.Int(), c.ChoiceText, c.Correct}
}

func EmptyChoice(questionIdx int) UiChoice {
	return UiChoice{-1, questionIdx, rand.Int(), "", false}
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

type CoursePageArgsStudent struct {
	NewCourseId int
	Editor      bool
	LoggedIn    bool
	Courses     []UiCourseStudent
}

type StudentPageArgs struct {
	Username string
}

func (r *Renderer) RenderStudentPage(w http.ResponseWriter, args StudentPageArgs) error {
	return r.templates["student.html"].ExecuteTemplate(w, "page.html", NewPageArgs(true, true, args))
}

func (r *Renderer) RenderTeacherCoursePage(w http.ResponseWriter, courses []UiCourse, newCourseId int) error {
	return r.templates["courses.html"].ExecuteTemplate(w, "page.html", NewPageArgs(true, true, CoursePageArgs{newCourseId, true, true, courses}))
}

func (r *Renderer) RenderStudentCoursePage(w http.ResponseWriter, courses []UiCourseStudent) error {
	return r.templates["courses.html"].ExecuteTemplate(w, "page.html", CoursePageArgsStudent{0, false, false, courses})
}

func (r *Renderer) RenderCreateCoursePage(w http.ResponseWriter) error {
	return r.templates["create_course.html"].ExecuteTemplate(w, "page.html", EmptyCourse())
}

func (r *Renderer) RenderCourseCreated(w http.ResponseWriter) error {
	return r.templates["create_course.html"].ExecuteTemplate(w, "created_course_response.html", nil)
}

func (r *Renderer) RenderEditCoursePage(w http.ResponseWriter, course UiCourse) error {
	return r.templates["create_course.html"].ExecuteTemplate(w, "page.html", course)
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
	Idx     int
	Content string
}

func NewUiContent(content Content) UiContent {
	return UiContent{content.Id, rand.Int(), content.Content}
}

func EmptyContent() UiContent {
	return UiContent{-1, rand.Int(), ""}
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
	return r.templates["edit_module.html"].ExecuteTemplate(w, "page.html", module)
}

func (r *Renderer) RenderModuleEdited(w http.ResponseWriter) error {
	return r.templates["edit_module.html"].ExecuteTemplate(w, "edited_module_response.html", nil)
}

type UiTakeModule struct {
	Module          UiModule
	BlockType       string
	Content         template.HTML
	BlockCount      int
	BlockIndex      int
	ChosenChoiceId  int
	CorrectChoiceId int
	Question        UiQuestion
	Explanation     template.HTML
}

func (r *Renderer) RenderTakeModulePage(w http.ResponseWriter, module UiTakeModule) error {
	return r.templates["take_module.html"].ExecuteTemplate(w, "page.html", module)
}

// Renders just the content, i.e. the header + content, not the full page.
func (r *Renderer) RenderTakeModule(w http.ResponseWriter, module UiTakeModule) error {
	return r.templates["take_module.html"].ExecuteTemplate(w, "content", module)
}

type UiSubmittedAnswer struct {
	Module          UiModule
	BlockCount      int
	BlockIndex      int
	ChosenChoiceId  int
	CorrectChoiceId int
	Question        UiQuestion
	Explanation     template.HTML
}

func (r *Renderer) RenderQuestionSubmitted(w http.ResponseWriter, module UiSubmittedAnswer) error {
	return r.templates["take_module.html"].ExecuteTemplate(w, "question_submitted", module)
}
