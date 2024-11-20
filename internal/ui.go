package internal

import (
	"html/template"
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

type CoursePageArgs struct {
	NewCourseId int
	Editor      bool
	Courses     []UiCourse
}

type CoursePageArgsStudent struct {
	NewCourseId int
	Editor      bool
	Courses     []UiCourseStudent
}

func (r *Renderer) RenderHomePage(w http.ResponseWriter) error {
	return r.templates["index.html"].ExecuteTemplate(w, "page.html", nil)
}

func (r *Renderer) RenderTeacherCoursePage(w http.ResponseWriter, courses []UiCourse, newCourseId int) error {
	return r.templates["courses.html"].ExecuteTemplate(w, "page.html", CoursePageArgs{newCourseId, true, courses})
}

func (r *Renderer) RenderStudentCoursePage(w http.ResponseWriter, courses []UiCourseStudent) error {
	return r.templates["courses.html"].ExecuteTemplate(w, "page.html", CoursePageArgsStudent{0, false, courses})
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

func (r *Renderer) RenderNewChoice(w http.ResponseWriter, choice UiChoice) error {
	return r.templates["add_element.html"].ExecuteTemplate(w, "add_element.html", choice)
}

func (r *Renderer) RenderEditModulePage(w http.ResponseWriter, module UiEditModule) error {
	return r.templates["edit_module.html"].ExecuteTemplate(w, "page.html", module)
}

func (r *Renderer) RenderModuleEdited(w http.ResponseWriter) error {
	return r.templates["edit_module.html"].ExecuteTemplate(w, "edited_module_response.html", nil)
}

type UiTakeModule struct {
	Module          UiModule
	QuestionCount   int
	QuestionIndex   int
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
	QuestionCount   int
	QuestionIndex   int
	ChosenChoiceId  int
	CorrectChoiceId int
	Question        UiQuestion
	Explanation     template.HTML
}

func (r *Renderer) RenderQuestionSubmitted(w http.ResponseWriter, module UiSubmittedAnswer) error {
	return r.templates["take_module.html"].ExecuteTemplate(w, "question_submitted", module)
}
