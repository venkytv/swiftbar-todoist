package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"text/template"

	"github.com/go-resty/resty/v2"
	"github.com/keybase/go-keychain"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

func (t *Task) Parse() {
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
}

func getTodoistApiToken() string {
	token := viper.GetString("api_token")
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
	_, err := request.
		SetQueryParams(map[string]string{
			"project_id": strconv.FormatInt(project_id, 10),
		}).
		SetResult(&tasks).
		Get(TODOIST_API + "/tasks")
	if err != nil {
		log.Fatal(err)
	}

	return tasks
}

func getProjectId(request *resty.Request) int64 {
	project := viper.GetString("project")
	log.Print("Looking up project ID for project: ", project)

	var projects []Project
	_, err := request.SetResult(&projects).Get(TODOIST_API + "/projects")
	if err != nil {
		log.Fatal(err)
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

func getTitle(ntasks int) Title {
	title_param := "title"
	if ntasks < 1 {
		title_param = "empty_title"
	}

	title_tmpl := viper.GetString(title_param)
	title_color := viper.GetString(title_param + "_color")

	if len(title_tmpl) < 1 {
		title_tmpl = viper.GetString("title")
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

func printTasks(wr io.Writer) {
	token := getTodoistApiToken()
	request := resty.New().R().
		SetHeader("Accept", "application/json").
		SetAuthToken(token)

	project_id := viper.GetInt64("project_id")
	if project_id < 1 {
		project_id = getProjectId(request)
	}

	tasks := getTasks(request, project_id)
	ntasks := len(tasks)

	title := getTitle(ntasks)
	body := make([]Task, ntasks)
	for i, _ := range tasks {
		tasks[i].Parse()
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
	tmpl_file := viper.GetString("output_template")
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

func main() {
	rootCmd := &cobra.Command{
		Run: func(cmd *cobra.Command, args []string) {
			printTasks(os.Stdout)
		},
	}
	rootCmd.Flags().String("project", "Inbox", "project to list tasks for")
	rootCmd.Flags().Int64("project-id", 0, "project ID (overrides project if set)")
	rootCmd.Flags().String("api-token", "", "todoist API token")
	rootCmd.Flags().String("title", ":{{ if (le .NumTasks 50) }}{{ .NumTasks }}{{ else }}ellipsis{{ end }}.circle.fill:", "menu bar title")
	rootCmd.Flags().String("title-color", "#DC143C", "title color")
	rootCmd.Flags().String("empty-title", "", "menu bar title when tasks empty")
	rootCmd.Flags().String("empty-title-color", "", "title color when tasks empty")
	rootCmd.Flags().String("output-template", "", "template file for output")

	viper.BindPFlag("project", rootCmd.Flags().Lookup("project"))
	viper.BindPFlag("project_id", rootCmd.Flags().Lookup("project-id"))
	viper.BindPFlag("api_token", rootCmd.Flags().Lookup("api-token"))
	viper.BindPFlag("title", rootCmd.Flags().Lookup("title"))
	viper.BindPFlag("title_color", rootCmd.Flags().Lookup("title-color"))
	viper.BindPFlag("empty_title", rootCmd.Flags().Lookup("empty-title"))
	viper.BindPFlag("empty_title_color", rootCmd.Flags().Lookup("empty-title-color"))
	viper.BindPFlag("output_template", rootCmd.Flags().Lookup("output-template"))

	viper.SetEnvPrefix("ST")
	viper.AutomaticEnv()

	rootCmd.Execute()
}
