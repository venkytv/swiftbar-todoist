package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"

	"github.com/go-resty/resty/v2"
	"github.com/keybase/go-keychain"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	TODOIST_API = "https://api.todoist.com/rest/v1"
)

type Project struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
}

type Task struct {
	Id          int64  `json:"id"`
	Content     string `json:"content"`
	Description string `json:"description"`
	Title       string
	Url         string
	Note        string
}

func (t *Task) Parse() {
	re := regexp.MustCompile(`\[([^\]]*)\]\(([^\)]*)\)\s*(.*)`)
	m := re.FindStringSubmatch(t.Content)
	if len(m) < 1 {
		log.Print("Regexp match fail: ", t.Content)
		t.Title = t.Content
	} else {
		t.Title = m[1]
		t.Url = m[2]
		t.Note = m[3]
	}
}

func (t *Task) title() string {
	log.Print("Task: ", t)
	if len(t.Title) < 1 {
		t.Parse()
	}
	return t.Title
}

func getTodoistApiToken() string {
	item, err := keychain.GetGenericPassword("todoist", "api-token", "", "")
	if err != nil {
		log.Fatal(err)
	}
	return string(item)
}

func getTasks(request *resty.Request, project_id int64) []Task {
	log.Print("Looks for tasks with project ID: ", project_id)

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

func printTasks() {
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

	body := make([]Task, ntasks)
	for i, _ := range tasks {
		tasks[i].Parse()
		body[i] = tasks[i]
	}

	var color, title, tooltip string

	icon := "icon"
	if ntasks < 1 {
		icon = "empty_icon"
		tooltip = "No pending tasks"
	} else if ntasks == 1 {
		tooltip = "One pending task"
	} else {
		tooltip = fmt.Sprintf("%d pending tasks", ntasks)
	}

	title = viper.GetString(icon)
	color = viper.GetString(icon + "_color")

	if len(title) < 1 {
		title = tooltip
		tooltip = ""
	}

	if len(color) > 0 {
		title += fmt.Sprintf(" | color=%s sfcolor=%s", color, color)
	}

	fmt.Println(title)
	fmt.Println("---")

	for _, t := range body {
		fmt.Println(t.Title)
	}
}

func main() {
	rootCmd := &cobra.Command{
		Run: func(cmd *cobra.Command, args []string) {
			printTasks()
		},
	}
	rootCmd.Flags().String("project", "Inbox", "project to list tasks for")
	rootCmd.Flags().Int64("project-id", 0, "project ID (overrides project if set)")
	rootCmd.Flags().String("icon", ":paperclip.badge.ellipsis:", "menu bar icon")
	rootCmd.Flags().String("icon-color", "#DC143C", "icon color")
	rootCmd.Flags().String("empty-icon", ":paperclip:", "menu bar icon when tasks empty")
	rootCmd.Flags().String("empty-icon-color", "", "icon color when tasks empty")

	viper.BindPFlag("project", rootCmd.Flags().Lookup("project"))
	viper.BindPFlag("project_id", rootCmd.Flags().Lookup("project-id"))
	viper.BindPFlag("icon", rootCmd.Flags().Lookup("icon"))
	viper.BindPFlag("icon_color", rootCmd.Flags().Lookup("icon-color"))
	viper.BindPFlag("empty_icon", rootCmd.Flags().Lookup("empty-icon"))
	viper.BindPFlag("empty_icon_color", rootCmd.Flags().Lookup("empty-icon-color"))

	viper.SetEnvPrefix("ST")
	viper.AutomaticEnv()

	rootCmd.Execute()
}
