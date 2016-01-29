package servertasks

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type TaskFunc func(*TaskBlock)
type TaskCreator func(*TaskBlock, *http.Request)

type TaskBlock struct {
	Name        string
	Start       int64
	Repeat      int
	RunTime     int
	Done        bool
	Success     bool
	SuccessText string
	ErrorText   string
	Func        TaskFunc `json:"-"`
}

var tasks []*TaskBlock = []*TaskBlock{}

func Start() {
	go func() {
		for {
			RunTasks()
			time.Sleep(time.Second * 5)
		}
	}()
}

func RunTasks() {
	for _, v := range tasks {
		if !v.Done {
			v.Func(v)
		}
	}
}

func GenHandler(fc TaskCreator) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		httpHandler(w, r, fc)
	}
}

func httpHandler(w http.ResponseWriter, r *http.Request, fc TaskCreator) {
	if r.Method == "GET" {
		getTasks(w, r)
	} else {
		putTask(w, r, fc)
	}
}

func getTasks(w http.ResponseWriter, r *http.Request) {
	b, err := json.Marshal(tasks)
	if err != nil {
		log.Println("cant marshal tasks " + err.Error())
		return
	}

	w.Write(b)
}

func putTask(w http.ResponseWriter, r *http.Request, fc TaskCreator) {
	err := r.ParseForm()
	if err != nil {
		log.Println("cant parseform")
		return
	}

	t := TaskBlock{}
	t.Name = r.FormValue("name")
	k, err := time.Parse(time.RFC822, r.FormValue("start"))
	if err != nil {
		t.Start = k.Unix()
	}
	fc(&t, r)
	tasks = append(tasks, &t)
}
