package ui

import (
	"fmt"
	"html/template"
	"net/http"
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

func (r *Renderer) RefreshTemplates() {
	r.templates = initTemplates(r.projectRootDir)
}

func initTemplates(projectRootDir string) map[string]*template.Template {
	funcMap := template.FuncMap{}
	filePaths := map[string][]string{
		"index.html":  {"page.html", "index.html"},
		"signup.html": {"page.html", "signup.html"},
	}
	templates := make(map[string]*template.Template)
	for name, paths := range filePaths {
		files := make([]string, len(paths))
		for i, path := range paths {
			files[i] = fmt.Sprintf("%s/template/%s", projectRootDir, path)
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

func newPageArgs(showNav, loggedIn bool, contentArgs interface{}) PageArgs {
	return PageArgs{showNav, loggedIn, contentArgs}
}

func (r *Renderer) RenderHomePage(w http.ResponseWriter, loggedIn bool) error {
	return r.templates["index.html"].ExecuteTemplate(w, "page.html", newPageArgs(true, loggedIn, nil))
}
