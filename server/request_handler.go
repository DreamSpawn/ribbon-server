package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/dreamspawn/ribbon-server/admin"
	"github.com/dreamspawn/ribbon-server/api"
	"github.com/dreamspawn/ribbon-server/config"
	"github.com/dreamspawn/ribbon-server/render"
	"github.com/dreamspawn/ribbon-server/server/page"
	"github.com/dreamspawn/ribbon-server/server/session"
	"github.com/dreamspawn/ribbon-server/server/svg"
	"github.com/dreamspawn/ribbon-server/translations"
	"github.com/dreamspawn/ribbon-server/user"
)

var admin_slug string
var fallback_lang string

func ConfigReady() {
	admin_slug = config.Get("admin_slug")
	fallback_lang = config.Get("fallback_lang")

	page.ConfigReady()
	session.ConfigReady()
}

type RequestHandler struct {
}

func (handler RequestHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var vars url.Values
	switch request.Method {
	case "GET":
		vars = request.URL.Query()
	case "POST":
		err := request.ParseForm()
		if err != nil {
			fmt.Print("Could not parse form data\n")
			panic(err)
		}
		vars = request.Form
	}

	// Handle API calls
	api_endpoint, found := strings.CutPrefix(request.URL.Path, "/api/")
	if found {
		api.Handle(api_endpoint, vars, *request, writer)
		return
	}

	// Handle standard page
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")

	page := new(page.Page)
	page.Lang = fallback_lang

	// Add all files in general folder and sub folders
	page.AddCSS("general")

	// Add JS files
	page.AddJS("jquery-3.7.1.js")
	page.AddJS("render.js")

	// Get general translations
	translations.Load("general", page.Lang)

	// Get the root template
	root_tmpl := render.LoadTemplate("root.tmpl")

	// Get session and user
	session := session.Open(writer, *request)
	session_user := session.GetUser()

	_, logout := vars["logout"]
	if logout {
		session.Delete(writer)
		session_user = nil
	}

	if session_user == nil {
		message := ""
		if request.Method == "POST" && !logout {
			session_user = user.TryLogin(vars)
			if session_user == nil {
				message = translations.Get(page.Lang, "general", "login_error")
			} else {
				session.SetUser(*session_user)
			}
		}
		if session_user == nil {
			// Login page
			page.AddTitle("[Login] Fastaval Ribbon Server")
			page.AddCSS("login.css")

			login_tmpl := render.LoadTemplate("login.tmpl")
			page_content := render.TemplateString(
				login_tmpl,
				map[string]string{
					"message":   message,
					"action":    request.URL.Path,
					"headline":  translations.Get(page.Lang, "general", "headline"),
					"user_name": translations.Get(page.Lang, "general", "user_name"),
					"password":  translations.Get(page.Lang, "general", "password"),
					"login":     translations.Get(page.Lang, "general", "login"),
				},
			)
			page.SetContent(page_content)
			render.WriteTemplate(root_tmpl, writer, page)
			return
		}
	}

	link := ""
	if admin_page, found := strings.CutPrefix(request.URL.Path, "/"+admin_slug); found && (admin_page == "" || strings.HasPrefix(admin_page, "/")) {
		// Admin pages
		standard_link_text := translations.Get(page.Lang, "general", "standard_link_text")
		link = fmt.Sprintf(`<a href="/">%s</a>`, standard_link_text)

		admin.BuildAdminPage(admin_page, page, *session_user)
		page.AddTitle("[Admin] Fastaval Ribbon Server")
	} else {
		// User pages
		headline := translations.Get(page.Lang, "general", "headline")

		// Add link to admin page if user is logged in as admin
		if session_user.IsAdmin {
			admin_link_text := translations.Get(page.Lang, "general", "admin_link_text")
			link = fmt.Sprintf(`<a href="/%s">%s</a>`, admin_slug, admin_link_text)
		}

		main_tmpl := render.LoadTemplate("main-content.tmpl")
		page_content := render.TemplateString(
			main_tmpl,
			map[string]string{
				"headline": headline,
			},
		)

		page_content += svg.GetSVGTest(vars)

		page.SetContent(page_content)
		page.AddTitle("Fastaval Ribbon Server")
	}

	user_header_tmpl := render.LoadTemplate("user-header.tmpl")
	user_header := render.TemplateString(user_header_tmpl, map[string]string{
		"name":   session_user.Name,
		"action": request.URL.Path,
		"logout": translations.Get(page.Lang, "general", "logout"),
		"link":   link,
	})

	page.Prepend(user_header)

	render.WriteTemplate(root_tmpl, writer, page)
}
