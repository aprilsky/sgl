package main

import (
	"fmt"
	"io"
	"net/http"
)

//myRouter是一个map  key:url  value:handler
var myRouter map[string]func(http.ResponseWriter, *http.Request)

func main() {
	myRouter = make(map[string]func(http.ResponseWriter, *http.Request))
	server := &http.Server{
		Addr:    ":8081",
		Handler: &myHander{},
	}
	myRouter["/"] = homeHandler
	myRouter["/one"] = oneHandler

	err := server.ListenAndServe()
	if err != nil {
		fmt.Println(err)
	}
}

type myHander struct{}

func (*myHander) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	url := r.URL.String()

	handler, ok := myRouter[url]
	if ok {
		handler(w, r)
		return
	}

	io.WriteString(w, "Not Found")
}

func regularHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "this is version regularHandler.")
}
func oneHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "this is version 1.")
}
func homeHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "this is home .")
}
