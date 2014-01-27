package main

import (
	. "config"
	"controller"
	"go.net/websocket"
	"log"
	"math/rand"
	"net/http"
	"path/filepath"
	"process"
	"runtime"
	"time"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	// 设置随机数种子
	rand.Seed(time.Now().Unix())
}

func main() {
	SavePid()
	// 服务静态文件
	http.Handle("/static/", http.FileServer(http.Dir(ROOT)))

	go ServeWebSocket()

	router := initRouter()
	http.Handle("/", router)
	log.Fatal(http.ListenAndServe(Config["host"], nil))
}

func ServeWebSocket() {
	http.Handle("/ws", websocket.Handler(controller.WsHandler))
	log.Fatal(http.ListenAndServe(Config["wshost"], nil))
}

// 保存PID
func SavePid() {
	pidFile := Config["pid"]
	if !filepath.IsAbs(Config["pid"]) {
		pidFile = ROOT + "/" + pidFile
	}
	// TODO：错误不处理
	process.SavePidTo(pidFile)
}
