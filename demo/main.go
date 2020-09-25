package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"

	"github.com/lwlwilliam/session"
	_ "github.com/lwlwilliam/session/providers/memory"
)

var globalSessions *session.Manager

func init() {
	globalSessions, _ = session.NewManager("memory", "gosessionid", 30) // 可以调小一点测试 GC
	go globalSessions.GC()
}

func login(w http.ResponseWriter, r *http.Request) {
	sess := globalSessions.SessionStart(w, r)

	r.ParseForm()
	if r.Method == "GET" {
		t, err := template.ParseFiles("./templates/login.gtpl")
		if err != nil {
			w.Write([]byte("Something wrong."))
			log.Println(err)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		t.Execute(w, sess.Get("username"))
		log.Println(sess.Get("username"))
	} else {
		if len(r.Form["username"][0]) == 0 {
			fmt.Fprintf(w, "%s", "the username can not be null")
		} else if m, _ := regexp.MatchString("^[a-z]{3}$", r.Form["username"][0]); !m { // 用户名只能由 3 个 a-z 之间的字符组成
			fmt.Fprintf(w, "%s", "the username is invalid")
		} else {
			sess.Set("username", r.Form["username"][0])
			fmt.Fprintf(w, "%s: %s", "log in successfully", template.HTMLEscapeString(r.Form["password"][0]))
		}

		fmt.Printf("username:%s; password:%s\n", r.Form["username"][0], template.HTMLEscapeString(r.Form["password"][0]))
	}
}

func main() {
	http.HandleFunc("/login", login)
	err := http.ListenAndServe(":9090", nil)
	if err != nil {
		log.Fatal(err)
	}
}
