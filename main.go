package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-resty/resty/v2"
	"github.com/keybase/go-keychain"
	"github.com/venkytv/go-config"
)

const (
	TODOIST_API = "https://api.todoist.com/rest/v1"
	OUTPUT_TMPL = `{{ .Title.Text }}{{ if .Title.Color }} | color={{ .Title.Color }}{{ end }} sfcolor={{ .Title.Color }}
---
{{ range .Tasks }}{{ .Name }}
{{ end }}`
)

type Project struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
}

type Task struct {
	Id          int64  `json:"id"`
	Content     string `json:"content"`
	Description string `json:"description"`
	Name        string
	Url         string
	Note        string
}

func (t *Task) Parse(cfg *config.Config) {
	re := regexp.MustCompile(`\[([^\]]*)\]\(([^\)]*)\)\s*(.*)`)
	m := re.FindStringSubmatch(t.Content)
	if len(m) < 1 {
		log.Print("Regexp match fail: ", t.Content)
		t.Name = t.Content
	} else {
		t.Name = m[1]
		t.Url = m[2]
		t.Note = m[3]
	}
	t.Name = strings.ReplaceAll(t.Name, "|", cfg.GetString("pipe-sub"))
}

func getTodoistApiToken(cfg *config.Config) string {
	token := cfg.GetString("api-token")
	if len(token) < 1 {
		item, err := keychain.GetGenericPassword("todoist", "api-token", "", "")
		if err != nil {
			log.Fatal(err)
		}
		token = string(item)
	}
	return token
}

func getTasks(request *resty.Request, project_id int64) []Task {
	log.Print("Looking for tasks with project ID: ", project_id)

	var tasks []Task
	resp, err := request.
		SetQueryParams(map[string]string{
			"project_id": strconv.FormatInt(project_id, 10),
		}).
		SetResult(&tasks).
		Get(TODOIST_API + "/tasks")
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		log.Fatal(resp.Status())
	}

	return tasks
}

func getProjectId(request *resty.Request, cfg *config.Config) int64 {
	project := cfg.GetString("project")
	log.Print("Looking up project ID for project: ", project)

	var projects []Project
	resp, err := request.SetResult(&projects).Get(TODOIST_API + "/projects")
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		log.Fatal(resp.Status())
	}

	for _, p := range projects {
		if p.Name == project {
			return p.Id
		}
	}

	log.Fatal("Project does not exist: ", project)
	return 0
}

type Title struct {
	Text  string
	Color string
}

func getTitle(ntasks int, cfg *config.Config) Title {
	title_param := "title"
	if ntasks < 1 {
		title_param = "empty-title"
	}

	title_tmpl := cfg.GetString(title_param)
	title_color := cfg.GetString(title_param + "-color")

	if len(title_tmpl) < 1 {
		title_tmpl = cfg.GetString("title")
	}

	// Default title
	title := fmt.Sprintf("Pending tasks: %d\n", ntasks)

	tmpl, err := template.New("title").Parse(title_tmpl)
	if err == nil {
		data := struct{ NumTasks int }{ntasks}
		var title_bytes bytes.Buffer
		err = tmpl.Execute(&title_bytes, data)
		if err == nil {
			title = title_bytes.String()
		}
	}

	return Title{title, title_color}
}

func printTasks(wr io.Writer, client *resty.Client, cfg *config.Config) {
	token := getTodoistApiToken(cfg)
	request := client.R().
		SetHeader("Accept", "application/json").
		SetAuthToken(token)

	project_id := cfg.GetInt64("project-id")
	if project_id < 1 {
		project_id = getProjectId(request, cfg)
	}

	tasks := getTasks(request, project_id)
	ntasks := len(tasks)

	title := getTitle(ntasks, cfg)
	body := make([]Task, ntasks)
	for i, _ := range tasks {
		tasks[i].Parse(cfg)
		body[i] = tasks[i]
	}

	data := struct {
		Title Title
		Tasks []Task
	}{
		title,
		tasks,
	}

	var tmpl_text string
	tmpl_file := cfg.GetString("output-template")
	if len(tmpl_file) > 0 {
		bytes, err := ioutil.ReadFile(tmpl_file)
		if err != nil {
			log.Fatal(err)
		}
		tmpl_text = string(bytes)
	} else {
		// Use built-in template
		tmpl_text = OUTPUT_TMPL
	}
	tmpl, err := template.New("output").Parse(tmpl_text)
	if err != nil {
		log.Fatal(err)
	}
	if err = tmpl.Execute(wr, data); err != nil {
		log.Fatal(err)
	}
}

func loadConfig() *config.Config {
	f := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	f.String("project", "Inbox", "project to list tasks for")
	f.Int64("project-id", 0, "project ID (overrides project if set)")
	f.String("api-token", "", "todoist API token")
	f.String("title", ":{{ if (le .NumTasks 50) }}{{ .NumTasks }}{{ else }}ellipsis{{ end }}.circle.fill:", "menu bar title")
	f.String("title-color", "#DC143C", "title color")
	f.String("empty-title", "", "menu bar title when tasks empty")
	f.String("empty-title-color", "", "title color when tasks empty")
	f.String("output-template", "", "template file for output")
	f.String("pipe-sub", "｜", "character to substitute for pipes in text")

	return config.Load(f, "ST")
}

func main() {
	cfg := loadConfig()
	client := resty.New()
	printTasks(os.Stdout, client, cfg)
}
