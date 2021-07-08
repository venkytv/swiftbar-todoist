package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/jarcoal/httpmock"
	"gotest.tools/assert"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		Name     string
		Task     Task
		WantName string
		WantUrl  string
		WantNote string
	}{
		{
			Name: "Normal",
			Task: Task{
				Id:          1,
				Content:     "[Title](http://example.com) Note",
				Description: "Description"},
			WantName: "Title",
			WantUrl:  "http://example.com",
			WantNote: "Note",
		},
		{
			Name: "ParseFail",
			Task: Task{
				Content: "This content does not parse",
			},
			WantName: "This content does not parse",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			task := tc.Task
			task.Parse()
			assert.Equal(t, tc.WantName, task.Name)
			assert.Equal(t, tc.WantUrl, task.Url)
			assert.Equal(t, tc.WantNote, task.Note)
		})
	}
}

func jsonFileResponder(statusCode int, fileName string) httpmock.Responder {
	bytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		panic(err)
	}
	resp := httpmock.NewStringResponse(statusCode, string(bytes))
	resp.Header.Set("Content-Type", "application/json")
	return httpmock.ResponderFromResponse(resp)
}

func TestPrintTasks(t *testing.T) {
	client := resty.New()
	httpmock.ActivateNonDefault(client.GetClient())
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", TODOIST_API+"/projects",
		jsonFileResponder(200, "testdata/projects.json"))
	httpmock.RegisterResponder("GET", TODOIST_API+"/tasks",
		jsonFileResponder(200, "testdata/tasks.json"))

	cfg := loadConfig()
	cfg.SetDefault("api-token", "XXXX")
	cfg.SetDefault("title", "{{.NumTasks}}")
	cfg.SetDefault("title-color", "")
	cfg.SetDefault("output-template", "testdata/test.tmpl")

	d, err := ioutil.ReadFile("testdata/test.out")
	if err != nil {
		panic(err)
	}
	wantOutput := string(d)

	output := bytes.NewBufferString("")
	printTasks(output, client, cfg)
	assert.Equal(t, wantOutput, output.String())
}

func TestMain(m *testing.M) {
	// Skip log messages during testing
	log.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}
