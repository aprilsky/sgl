package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/printer"
	"go/scanner"
	"go/token"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	path "path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Command struct {
	// Run runs the command.
	// The args are the arguments after the command name.
	Run func(cmd *Command, args []string)

	// UsageLine is the one-line usage message.
	// The first word in the line is taken to be the command name.
	UsageLine string

	// Short is the short description shown in the 'go help' output.
	Short string

	// Long is the long message shown in the 'go help <this-command>' output.
	Long string

	// Flag is a set of flags specific to this command.
	Flag flag.FlagSet

	// CustomFlags indicates that the command will do its own
	// flag parsing.
	CustomFlags bool
}

// Name returns the command's name: the first word in the usage line.
func (c *Command) Name() string {
	name := c.UsageLine
	i := strings.Index(name, " ")
	if i >= 0 {
		name = name[:i]
	}
	return name
}

func (c *Command) Usage() {
	fmt.Fprintf(os.Stderr, "usage: %s\n\n", c.UsageLine)
	fmt.Fprintf(os.Stderr, "%s\n", strings.TrimSpace(c.Long))
	os.Exit(2)
}

// Runnable reports whether the command can be run; otherwise
// it is a documentation pseudo-command such as importpath.
func (c *Command) Runnable() bool {
	return c.Run != nil
}

var commands = []*Command{
	cmdNew,
	cmdRun,
	cmdPack,
	cmdApiapp,
	//	cmdRouter,
	//cmdReStart,
}

func main() {
	flag.Usage = usage
	flag.Parse()
	log.SetFlags(0)

	args := flag.Args()
	if len(args) < 1 {
		usage()
	}

	if args[0] == "help" {
		help(args[1:])
		return
	}

	for _, cmd := range commands {
		if cmd.Name() == args[0] && cmd.Run != nil {
			cmd.Flag.Usage = func() { cmd.Usage() }
			if cmd.CustomFlags {
				args = args[1:]
			} else {
				cmd.Flag.Parse(args[1:])
				args = cmd.Flag.Args()
			}
			cmd.Run(cmd, args)
			os.Exit(2)
			return
		}
	}

	fmt.Fprintf(os.Stderr, "bee: unknown subcommand %q\nRun 'bee help' for usage.\n", args[0])
	os.Exit(2)
}

var usageTemplate = `Bee is a tool for managing beego framework.

Usage:

	bee command [arguments]

The commands are:
{{range .}}{{if .Runnable}}
    {{.Name | printf "%-11s"}} {{.Short}}{{end}}{{end}}

Use "bee help [command]" for more information about a command.

Additional help topics:
{{range .}}{{if not .Runnable}}
    {{.Name | printf "%-11s"}} {{.Short}}{{end}}{{end}}

Use "bee help [topic]" for more information about that topic.

`

var helpTemplate = `{{if .Runnable}}usage: bee {{.UsageLine}}

{{end}}{{.Long | trim}}
`

func usage() {
	tmpl(os.Stdout, usageTemplate, commands)
	os.Exit(2)
}

func tmpl(w io.Writer, text string, data interface{}) {
	t := template.New("top")
	t.Funcs(template.FuncMap{"trim": strings.TrimSpace})
	template.Must(t.Parse(text))
	err := t.Execute(w, data)
	if err != nil {
		panic(err)
	}
}

func help(args []string) {
	if len(args) == 0 {
		usage()
		// not exit 2: succeeded at 'go help'.
		return
	}
	if len(args) != 1 {
		fmt.Fprintf(os.Stdout, "usage: bee help command\n\nToo many arguments given.\n")
		os.Exit(2) // failed at 'bee help'
	}

	arg := args[0]

	for _, cmd := range commands {
		if cmd.Name() == arg {
			tmpl(os.Stdout, helpTemplate, cmd)
			// not exit 2: succeeded at 'go help cmd'.
			return
		}
	}

	fmt.Fprintf(os.Stdout, "Unknown help topic %#q.  Run 'bee help'.\n", arg)
	os.Exit(2) // failed at 'bee help cmd'
}

var cmdApiapp = &Command{
	// CustomFlags: true,
	UsageLine: "api [appname]",
	Short:     "create an api application base on beego framework",
	Long: `
create an api application base on beego framework

In the current path, will create a folder named [appname]

In the appname folder has the follow struct:

	├── conf
	│   └── app.conf
	├── controllers
	│   └── default.go
	├── main.go
	└── models
	    └── object.go             

`,
}

var apiconf = `
appname = {{.Appname}}
httpport = 8080
runmode = dev
autorender = false
copyrequestbody = true
`
var apiMaingo = `package main

import (
	"github.com/astaxie/beego"
	"{{.Appname}}/controllers"
)

//		Objects

//	URL					HTTP Verb				Functionality
//	/object				POST					Creating Objects
//	/object/<objectId>	GET						Retrieving Objects
//	/object/<objectId>	PUT						Updating Objects
//	/object				GET						Queries
//	/object/<objectId>	DELETE					Deleting Objects

func main() {
	beego.RESTRouter("/object", &controllers.ObejctController{})
	beego.Router("/ping", &controllers.ObejctController{},"get:Ping")
	beego.Run()
}
`
var apiModels = `package models

import (
	"errors"
	"strconv"
	"time"
)

var (
	Objects map[string]*Object
)

type Object struct {
	ObjectId   string
	Score      int64
	PlayerName string
}

func init() {
	Objects = make(map[string]*Object)
	Objects["hjkhsbnmn123"] = &Object{"hjkhsbnmn123", 100, "astaxie"}
	Objects["mjjkxsxsaa23"] = &Object{"mjjkxsxsaa23", 101, "someone"}
}

func AddOne(object Object) (ObjectId string) {
	object.ObjectId = "astaxie" + strconv.FormatInt(time.Now().UnixNano(), 10)
	Objects[object.ObjectId] = &object
	return object.ObjectId
}

func GetOne(ObjectId string) (object *Object, err error) {
	if v, ok := Objects[ObjectId]; ok {
		return v, nil
	}
	return nil, errors.New("ObjectId Not Exist")
}

func GetAll() map[string]*Object {
	return Objects
}

func Update(ObjectId string, Score int64) (err error) {
	if v, ok := Objects[ObjectId]; ok {
		v.Score = Score
		return nil
	}
	return errors.New("ObjectId Not Exist")
}

func Delete(ObjectId string) {
	delete(Objects, ObjectId)
}
`

var apiControllers = `package controllers

import (
	"encoding/json"
	"github.com/astaxie/beego"
	"{{.Appname}}/models"
)

type ResponseInfo struct {
}

type ObejctController struct {
	beego.Controller
}

func (this *ObejctController) Post() {
	var ob models.Object
	json.Unmarshal(this.Ctx.RequestBody, &ob)
	objectid := models.AddOne(ob)
	this.Data["json"] = map[string]string{"ObjectId": objectid}
	this.ServeJson()
}

func (this *ObejctController) Get() {
	objectId := this.Ctx.Params[":objectId"]
	if objectId != "" {
		ob, err := models.GetOne(objectId)
		if err != nil {
			this.Data["json"] = err
		} else {
			this.Data["json"] = ob
		}
	} else {
		obs := models.GetAll()
		this.Data["json"] = obs
	}
	this.ServeJson()
}

func (this *ObejctController) Put() {
	objectId := this.Ctx.Params[":objectId"]
	var ob models.Object
	json.Unmarshal(this.Ctx.RequestBody, &ob)

	err := models.Update(objectId, ob.Score)
	if err != nil {
		this.Data["json"] = err
	} else {
		this.Data["json"] = "update success!"
	}
	this.ServeJson()
}

func (this *ObejctController) Delete() {
	objectId := this.Ctx.Params[":objectId"]
	models.Delete(objectId)
	this.Data["json"] = "delete success!"
	this.ServeJson()
}

func (this *ObejctController) Ping() {
    this.Ctx.WriteString("pong")
}

`

var apiTests = `package tests

import (
    "testing"
	beetest "github.com/astaxie/beego/testing"
	"io/ioutil"
)

func TestHelloWorld(t *testing.T) {
	request:=beetest.Get("/ping")
	response,_:=request.Response()
	defer response.Body.Close()
	contents, _ := ioutil.ReadAll(response.Body)
	if string(contents)!="pong"{
        t.Errorf("response sould be pong")
    }
}

`

func init() {
	cmdApiapp.Run = createapi
}

func createapi(cmd *Command, args []string) {
	if len(args) != 1 {
		fmt.Println("error args")
		os.Exit(2)
	}
	apppath, packpath, err := checkEnv(args[0])
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	os.MkdirAll(apppath, 0755)
	fmt.Println("create app folder:", apppath)
	os.Mkdir(path.Join(apppath, "conf"), 0755)
	fmt.Println("create conf:", path.Join(apppath, "conf"))
	os.Mkdir(path.Join(apppath, "controllers"), 0755)
	fmt.Println("create controllers:", path.Join(apppath, "controllers"))
	os.Mkdir(path.Join(apppath, "models"), 0755)
	fmt.Println("create models:", path.Join(apppath, "models"))
	os.Mkdir(path.Join(apppath, "tests"), 0755)
	fmt.Println("create tests:", path.Join(apppath, "tests"))

	fmt.Println("create conf app.conf:", path.Join(apppath, "conf", "app.conf"))
	writetofile(path.Join(apppath, "conf", "app.conf"),
		strings.Replace(apiconf, "{{.Appname}}", args[0], -1))

	fmt.Println("create controllers default.go:", path.Join(apppath, "controllers", "default.go"))
	writetofile(path.Join(apppath, "controllers", "default.go"),
		strings.Replace(apiControllers, "{{.Appname}}", packpath, -1))

	fmt.Println("create tests default.go:", path.Join(apppath, "tests", "default_test.go"))
	writetofile(path.Join(apppath, "tests", "default_test.go"),
		apiTests)

	fmt.Println("create models object.go:", path.Join(apppath, "models", "object.go"))
	writetofile(path.Join(apppath, "models", "object.go"), apiModels)

	fmt.Println("create main.go:", path.Join(apppath, "main.go"))
	writetofile(path.Join(apppath, "main.go"),
		strings.Replace(apiMaingo, "{{.Appname}}", packpath, -1))
}

func checkEnv(appname string) (apppath, packpath string, err error) {
	curpath, err := os.Getwd()
	if err != nil {
		return
	}

	gopath := os.Getenv("GOPATH")
	Debugf("gopath:%s", gopath)
	if gopath == "" {
		err = fmt.Errorf("you should set GOPATH in the env")
		return
	}

	appsrcpath := ""
	haspath := false
	wgopath := path.SplitList(gopath)
	for _, wg := range wgopath {
		wg = path.Join(wg, "src")

		if path.HasPrefix(strings.ToLower(curpath), strings.ToLower(wg)) {
			haspath = true
			appsrcpath = wg
			break
		}
	}

	if !haspath {
		err = fmt.Errorf("can't create application outside of GOPATH `%s`\n"+
			"you first should `cd $GOPATH%ssrc` then use create\n", gopath, string(path.Separator))
		return
	}
	apppath = path.Join(curpath, appname)

	if _, e := os.Stat(apppath); os.IsNotExist(e) == false {
		err = fmt.Errorf("path `%s` exists, can not create app without remove it\n", apppath)
		return
	}
	packpath = strings.Join(strings.Split(apppath[len(appsrcpath)+1:], string(path.Separator)), "/")
	return
}

const (
	notPredeclared = iota
	predeclaredType
	predeclaredConstant
	predeclaredFunction
)

// predeclared represents the set of all predeclared identifiers.
var predeclared = map[string]int{
	"bool":       predeclaredType,
	"byte":       predeclaredType,
	"complex128": predeclaredType,
	"complex64":  predeclaredType,
	"error":      predeclaredType,
	"float32":    predeclaredType,
	"float64":    predeclaredType,
	"int16":      predeclaredType,
	"int32":      predeclaredType,
	"int64":      predeclaredType,
	"int8":       predeclaredType,
	"int":        predeclaredType,
	"rune":       predeclaredType,
	"string":     predeclaredType,
	"uint16":     predeclaredType,
	"uint32":     predeclaredType,
	"uint64":     predeclaredType,
	"uint8":      predeclaredType,
	"uint":       predeclaredType,
	"uintptr":    predeclaredType,

	"true":  predeclaredConstant,
	"false": predeclaredConstant,
	"iota":  predeclaredConstant,
	"nil":   predeclaredConstant,

	"append":  predeclaredFunction,
	"cap":     predeclaredFunction,
	"close":   predeclaredFunction,
	"complex": predeclaredFunction,
	"copy":    predeclaredFunction,
	"delete":  predeclaredFunction,
	"imag":    predeclaredFunction,
	"len":     predeclaredFunction,
	"make":    predeclaredFunction,
	"new":     predeclaredFunction,
	"panic":   predeclaredFunction,
	"print":   predeclaredFunction,
	"println": predeclaredFunction,
	"real":    predeclaredFunction,
	"recover": predeclaredFunction,
}

const (
	ExportLinkAnnotation AnnotationKind = iota
	AnchorAnnotation
	CommentAnnotation
	PackageLinkAnnotation
	BuiltinAnnotation
)

// annotationVisitor collects annotations.
type annotationVisitor struct {
	annotations []Annotation
}

func (v *annotationVisitor) add(kind AnnotationKind, importPath string) {
	v.annotations = append(v.annotations, Annotation{Kind: kind, ImportPath: importPath})
}

func (v *annotationVisitor) ignoreName() {
	v.add(-1, "")
}

func (v *annotationVisitor) Visit(n ast.Node) ast.Visitor {
	switch n := n.(type) {
	case *ast.TypeSpec:
		v.ignoreName()
		ast.Walk(v, n.Type)
	case *ast.FuncDecl:
		if n.Recv != nil {
			ast.Walk(v, n.Recv)
		}
		v.ignoreName()
		ast.Walk(v, n.Type)
	case *ast.Field:
		for _ = range n.Names {
			v.ignoreName()
		}
		ast.Walk(v, n.Type)
	case *ast.ValueSpec:
		for _ = range n.Names {
			v.add(AnchorAnnotation, "")
		}
		if n.Type != nil {
			ast.Walk(v, n.Type)
		}
		for _, x := range n.Values {
			ast.Walk(v, x)
		}
	case *ast.Ident:
		switch {
		case n.Obj == nil && predeclared[n.Name] != notPredeclared:
			v.add(BuiltinAnnotation, "")
		case n.Obj != nil && ast.IsExported(n.Name):
			v.add(ExportLinkAnnotation, "")
		default:
			v.ignoreName()
		}
	case *ast.SelectorExpr:
		if x, _ := n.X.(*ast.Ident); x != nil {
			if obj := x.Obj; obj != nil && obj.Kind == ast.Pkg {
				if spec, _ := obj.Decl.(*ast.ImportSpec); spec != nil {
					if path, err := strconv.Unquote(spec.Path.Value); err == nil {
						v.add(PackageLinkAnnotation, path)
						if path == "C" {
							v.ignoreName()
						} else {
							v.add(ExportLinkAnnotation, path)
						}
						return nil
					}
				}
			}
		}
		ast.Walk(v, n.X)
		v.ignoreName()
	default:
		return v
	}
	return nil
}

func printDecl(decl ast.Node, fset *token.FileSet, buf []byte) (Code, []byte) {
	v := &annotationVisitor{}
	ast.Walk(v, decl)

	buf = buf[:0]
	err := (&printer.Config{Mode: printer.UseSpaces, Tabwidth: 4}).Fprint(sliceWriter{&buf}, fset, decl)
	if err != nil {
		return Code{Text: err.Error()}, buf
	}

	var annotations []Annotation
	var s scanner.Scanner
	fset = token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(buf))
	s.Init(file, buf, nil, scanner.ScanComments)
loop:
	for {
		pos, tok, lit := s.Scan()
		switch tok {
		case token.EOF:
			break loop
		case token.COMMENT:
			p := file.Offset(pos)
			e := p + len(lit)
			if p > math.MaxInt16 || e > math.MaxInt16 {
				break loop
			}
			annotations = append(annotations, Annotation{Kind: CommentAnnotation, Pos: int16(p), End: int16(e)})
		case token.IDENT:
			if len(v.annotations) == 0 {
				// Oops!
				break loop
			}
			annotation := v.annotations[0]
			v.annotations = v.annotations[1:]
			if annotation.Kind == -1 {
				continue
			}
			p := file.Offset(pos)
			e := p + len(lit)
			if p > math.MaxInt16 || e > math.MaxInt16 {
				break loop
			}
			annotation.Pos = int16(p)
			annotation.End = int16(e)
			if len(annotations) > 0 && annotation.Kind == ExportLinkAnnotation {
				prev := annotations[len(annotations)-1]
				if prev.Kind == PackageLinkAnnotation &&
					prev.ImportPath == annotation.ImportPath &&
					prev.End+1 == annotation.Pos {
					// merge with previous
					annotation.Pos = prev.Pos
					annotations[len(annotations)-1] = annotation
					continue loop
				}
			}
			annotations = append(annotations, annotation)
		}
	}
	return Code{Text: string(buf), Annotations: annotations}, buf
}

type AnnotationKind int16

type Annotation struct {
	Pos, End   int16
	Kind       AnnotationKind
	ImportPath string
}

type Code struct {
	Text        string
	Annotations []Annotation
}

func commentAnnotations(src string) []Annotation {
	var annotations []Annotation
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	s.Init(file, []byte(src), nil, scanner.ScanComments)
	for {
		pos, tok, lit := s.Scan()
		switch tok {
		case token.EOF:
			return annotations
		case token.COMMENT:
			p := file.Offset(pos)
			e := p + len(lit)
			if p > math.MaxInt16 || e > math.MaxInt16 {
				return annotations
			}
			annotations = append(annotations, Annotation{Kind: CommentAnnotation, Pos: int16(p), End: int16(e)})
		}
	}
	return nil
}

type sliceWriter struct{ p *[]byte }

func (w sliceWriter) Write(p []byte) (int, error) {
	*w.p = append(*w.p, p...)
	return len(p), nil
}

var cmdNew = &Command{
	UsageLine: "new [appname]",
	Short:     "create an application base on beego framework",
	Long: `
create an application base on beego framework,

which in the current path with folder named [appname].

The [appname] folder has following structure:

    |- main.go
    |- conf
        |-  app.conf
    |- controllers
         |- default.go
    |- models
    |- static
         |- js
         |- css
         |- img             
    |- views
        index.tpl                   

`,
}

func init() {
	cmdNew.Run = createApp
}

func createApp(cmd *Command, args []string) {
	curpath, _ := os.Getwd()
	if len(args) != 1 {
		colorLog("[ERRO] Argument [appname] is missing\n")
		os.Exit(2)
	}

	gopath := os.Getenv("GOPATH")
	Debugf("gopath:%s", gopath)
	if gopath == "" {
		colorLog("[ERRO] $GOPATH not found\n")
		colorLog("[HINT] Set $GOPATH in your environment vairables\n")
		os.Exit(2)
	}
	haspath := false
	appsrcpath := ""

	wgopath := path.SplitList(gopath)
	for _, wg := range wgopath {
		wg, _ = path.EvalSymlinks(path.Join(wg, "src"))

		if strings.HasPrefix(strings.ToLower(curpath), strings.ToLower(wg)) {
			haspath = true
			appsrcpath = wg
			break
		}
	}

	if !haspath {
		colorLog("[ERRO] Unable to create an application outside of $GOPATH(%s)\n", gopath)
		colorLog("[HINT] Change your work directory by `cd ($GOPATH%ssrc)`\n", string(path.Separator))
		os.Exit(2)
	}

	apppath := path.Join(curpath, args[0])

	if _, err := os.Stat(apppath); os.IsNotExist(err) == false {
		fmt.Printf("[ERRO] Path(%s) has alreay existed\n", apppath)
		os.Exit(2)
	}

	fmt.Println("[INFO] Creating application...")

	os.MkdirAll(apppath, 0755)
	fmt.Println(apppath + string(path.Separator))
	os.Mkdir(path.Join(apppath, "conf"), 0755)
	fmt.Println(path.Join(apppath, "conf") + string(path.Separator))
	os.Mkdir(path.Join(apppath, "controllers"), 0755)
	fmt.Println(path.Join(apppath, "controllers") + string(path.Separator))
	os.Mkdir(path.Join(apppath, "models"), 0755)
	fmt.Println(path.Join(apppath, "models") + string(path.Separator))
	os.Mkdir(path.Join(apppath, "static"), 0755)
	fmt.Println(path.Join(apppath, "static") + string(path.Separator))
	os.Mkdir(path.Join(apppath, "static", "js"), 0755)
	fmt.Println(path.Join(apppath, "static", "js") + string(path.Separator))
	os.Mkdir(path.Join(apppath, "static", "css"), 0755)
	fmt.Println(path.Join(apppath, "static", "css") + string(path.Separator))
	os.Mkdir(path.Join(apppath, "static", "img"), 0755)
	fmt.Println(path.Join(apppath, "static", "img") + string(path.Separator))
	fmt.Println(path.Join(apppath, "views") + string(path.Separator))
	os.Mkdir(path.Join(apppath, "views"), 0755)
	fmt.Println(path.Join(apppath, "conf", "app.conf"))
	writetofile(path.Join(apppath, "conf", "app.conf"), strings.Replace(appconf, "{{.Appname}}", args[0], -1))

	fmt.Println(path.Join(apppath, "controllers", "default.go"))
	writetofile(path.Join(apppath, "controllers", "default.go"), controllers)

	fmt.Println(path.Join(apppath, "views", "index.tpl"))
	writetofile(path.Join(apppath, "views", "index.tpl"), indextpl)

	fmt.Println(path.Join(apppath, "main.go"))
	writetofile(path.Join(apppath, "main.go"), strings.Replace(maingo, "{{.Appname}}", strings.Join(strings.Split(apppath[len(appsrcpath)+1:], string(path.Separator)), string(path.Separator)), -1))

	colorLog("[SUCC] New application successfully created!\n")
}

var appconf = `appname = {{.Appname}}
httpport = 8080
runmode = dev
`

var maingo = `package main

import (
	"{{.Appname}}/controllers"
	"github.com/astaxie/beego"
)

func main() {
	beego.Router("/", &controllers.MainController{})
	beego.Run()
}

`
var controllers = `package controllers

import (
	"github.com/astaxie/beego"
)

type MainController struct {
	beego.Controller
}

func (this *MainController) Get() {
	this.Data["Website"] = "beego.me"
	this.Data["Email"] = "astaxie@gmail.com"
	this.TplNames = "index.tpl"
}
`

var indextpl = `<!DOCTYPE html>

<html>
  	<head>
    	<title>Beego</title>
    	<meta http-equiv="Content-Type" content="text/html; charset=utf-8">
  	</head>
	
	<style type="text/css">
		body {
			margin: 0px;
			font-family: "Helvetica Neue",Helvetica,Arial,sans-serif;
			font-size: 14px;
			line-height: 20px;
			color: rgb(51, 51, 51);
			background-color: rgb(255, 255, 255);
		}

		.hero-unit {
			padding: 60px;
			margin-bottom: 30px;
			border-radius: 6px 6px 6px 6px;
		}

		.container {
			width: 940px;
			margin-right: auto;
			margin-left: auto;
		}

		.row {
			margin-left: -20px;
		}

		h1 {
			margin: 10px 0px;
			font-family: inherit;
			font-weight: bold;
			text-rendering: optimizelegibility;
		}

		.hero-unit h1 {
			margin-bottom: 0px;
			font-size: 60px;
			line-height: 1;
			letter-spacing: -1px;
			color: inherit;
		}

		.description {
		    padding-top: 5px;
		    padding-left: 5px;
		    font-size: 18px;
		    font-weight: 200;
		    line-height: 30px;
		    color: inherit;
		}

		p {
		    margin: 0px 0px 10px;
		}
	</style>
  	
  	<body>
  		<header class="hero-unit" style="background-color:#A9F16C">
			<div class="container">
			<div class="row">
			  <div class="hero-text">
			    <h1>Welcome to Beego!</h1>
			    <p class="description">
			    	Beego is a simple & powerful Go web framework which is inspired by tornado and sinatra.
			    <br />
			    	Official website: <a href="http://{{.Website}}">{{.Website}}</a>
			    <br />
			    	Contact me: {{.Email}}</a>
			    </p>
			  </div>
			</div>
			</div>
		</header>
	</body>
</html>
`

func writetofile(filename, content string) {
	f, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	f.WriteString(content)
}

var cmdRun = &Command{
	UsageLine: "run [appname]",
	Short:     "run the app which can hot compile",
	Long: `
start the appname throw exec.Command

then start a inotify watch for current dir
										
when the file has changed bee will auto go build and restart the app

	file changed
	     |
  check if it's go file
	     |
     yes     no
      |       |
 go build    do nothing
     |
 restart app   
`,
}

var defaultJson = `
{
	"go_install": false,
	"dir_structure":{
		"controllers": "",
		"models": "",
		"others": []
	},
	"main_files":{
		"main.go": "",
		"others": []
	}
}
`

func init() {
	cmdRun.Run = runApp
}

var appname string
var conf struct {
	// Indicates whether execute "go install" before "go build".
	GoInstall bool `json:"go_install"`

	DirStruct struct {
		Controllers string
		Models      string
		Others      []string // Other directories.
	} `json:"dir_structure"`

	MainFiles struct {
		Main   string   `json:"main.go"`
		Others []string // Others files of package main.
	} `json:"main_files"`
}

func runApp(cmd *Command, args []string) {
	exit := make(chan bool)
	if len(args) != 1 {
		colorLog("[ERRO] Cannot start running[ %s ]\n",
			"argument 'appname' is missing")
		os.Exit(2)
	}
	crupath, _ := os.Getwd()
	Debugf("current path:%s\n", crupath)

	err := loadConfig()
	if err != nil {
		colorLog("[ERRO] Fail to parse bee.json[ %s ]", err)
	}
	var paths []string
	paths = append(paths,
		path.Join(crupath, conf.DirStruct.Controllers),
		path.Join(crupath, conf.DirStruct.Models),
		path.Join(crupath, "./")) // Current path.
	// Because monitor files has some issues, we watch current directory
	// and ignore non-go files.
	paths = append(paths, conf.DirStruct.Others...)
	paths = append(paths, conf.MainFiles.Others...)

	NewWatcher(paths)
	appname = args[0]
	Autobuild()
	for {
		select {
		case <-exit:
			runtime.Goexit()
		}
	}
}

// loadConfig loads customized configuration.
func loadConfig() error {
	f, err := os.Open("bee.json")
	if err != nil {
		// Use default.
		err = json.Unmarshal([]byte(defaultJson), &conf)
		if err != nil {
			return err
		}
	} else {
		defer f.Close()
		colorLog("[INFO] Detected bee.json\n")
		d := json.NewDecoder(f)
		err = d.Decode(&conf)
		if err != nil {
			return err
		}
	}
	// Set variables.
	if len(conf.DirStruct.Controllers) == 0 {
		conf.DirStruct.Controllers = "controllers"
	}
	if len(conf.DirStruct.Models) == 0 {
		conf.DirStruct.Models = "models"
	}
	if len(conf.MainFiles.Main) == 0 {
		conf.MainFiles.Main = "main.go"
	}
	return nil
}

var cmdPack = &Command{
	CustomFlags: true,
	UsageLine:   "pack",
	Short:       "compress an beego project",
	Long: `
compress an beego project

-p        app path. default is current path
-b        build specify platform app. default true
-o        compressed file output dir. default use current path
-f        format. [ tar.gz / zip ]. default tar.gz. note: zip doesn't support embed symlink, skip it
-exp      path exclude prefix
-exs      path exclude suffix. default: .go:.DS_Store:.tmp
          all path use : as separator
-fs       follow symlink. default false
-ss       skip symlink. default false
          default embed symlink into compressed file
-v        verbose
`,
}

var (
	appPath  string
	excludeP string
	excludeS string
	outputP  string
	fsym     bool
	ssym     bool
	build    bool
	verbose  bool
	format   string
)

func init() {
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	fs.StringVar(&appPath, "p", "", "")
	fs.StringVar(&excludeP, "exp", "", "")
	fs.StringVar(&excludeS, "exs", ".go:.DS_Store:.tmp", "")
	fs.StringVar(&outputP, "o", "", "")
	fs.BoolVar(&build, "b", true, "")
	fs.BoolVar(&fsym, "fs", false, "")
	fs.BoolVar(&ssym, "ss", false, "")
	fs.BoolVar(&verbose, "v", false, "")
	fs.StringVar(&format, "f", "tar.gz", "")
	cmdPack.Flag = *fs
	cmdPack.Run = packApp
}

func exitPrint(con string) {
	fmt.Fprintln(os.Stderr, con)
	os.Exit(2)
}

type walker interface {
	isExclude(string) bool
	isEmpty(string) bool
	relName(string) string
	virPath(string) string
	compress(string, string, os.FileInfo) (bool, error)
	walkRoot(string) error
}

type byName []os.FileInfo

func (f byName) Len() int           { return len(f) }
func (f byName) Less(i, j int) bool { return f[i].Name() < f[j].Name() }
func (f byName) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }

type walkFileTree struct {
	wak           walker
	prefix        string
	excludePrefix []string
	excludeSuffix []string
	allfiles      map[string]bool
}

func (wft *walkFileTree) setPrefix(prefix string) {
	wft.prefix = prefix
}

func (wft *walkFileTree) isExclude(name string) bool {
	if name == "" {
		return true
	}
	for _, prefix := range wft.excludePrefix {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	for _, suffix := range wft.excludeSuffix {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func (wft *walkFileTree) isEmpty(fpath string) bool {
	fh, _ := os.Open(fpath)
	defer fh.Close()
	infos, _ := fh.Readdir(-1)
	for _, fi := range infos {
		fp := path.Join(fpath, fi.Name())
		if wft.isExclude(wft.virPath(fp)) {
			continue
		}
		if fi.Mode()&os.ModeSymlink > 0 {
			continue
		}
		if fi.IsDir() && wft.isEmpty(fp) {
			continue
		}
		return false
	}
	return true
}

func (wft *walkFileTree) relName(fpath string) string {
	name, _ := path.Rel(wft.prefix, fpath)
	return name
}

func (wft *walkFileTree) virPath(fpath string) string {
	name := fpath[len(wft.prefix):]
	if name == "" {
		return ""
	}
	name = name[1:]
	return name
}

func (wft *walkFileTree) readDir(dirname string) ([]os.FileInfo, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	list, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	sort.Sort(byName(list))
	return list, nil
}

func (wft *walkFileTree) walkLeaf(fpath string, fi os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if fpath == outputP {
		return nil
	}

	if fi.IsDir() {
		return nil
	}

	name := wft.virPath(fpath)
	if wft.isExclude(name) {
		return nil
	}

	if ssym && fi.Mode()&os.ModeSymlink > 0 {
		return nil
	}

	if wft.allfiles[name] {
		return nil
	}

	if added, err := wft.wak.compress(name, fpath, fi); added {
		if verbose {
			fmt.Printf("Compressed: %s\n", name)
		}
		wft.allfiles[name] = true
		return err
	} else {
		return err
	}
}

func (wft *walkFileTree) iterDirectory(fpath string, fi os.FileInfo) error {
	doFSym := fsym && fi.Mode()&os.ModeSymlink > 0
	if doFSym {
		nfi, err := os.Stat(fpath)
		if os.IsNotExist(err) {
			return nil
		}
		fi = nfi
	}

	err := wft.walkLeaf(fpath, fi, nil)
	if err != nil {
		if fi.IsDir() && err == path.SkipDir {
			return nil
		}
		return err
	}

	if !fi.IsDir() {
		return nil
	}

	list, err := wft.readDir(fpath)
	if err != nil {
		return wft.walkLeaf(fpath, fi, err)
	}

	for _, fileInfo := range list {
		err = wft.iterDirectory(path.Join(fpath, fileInfo.Name()), fileInfo)
		if err != nil {
			if !fileInfo.IsDir() || err != path.SkipDir {
				return err
			}
		}
	}
	return nil
}

func (wft *walkFileTree) walkRoot(root string) error {
	wft.prefix = root
	fi, err := os.Stat(root)
	if err != nil {
		return err
	}
	return wft.iterDirectory(root, fi)
}

type tarWalk struct {
	walkFileTree
	tw *tar.Writer
}

func (wft *tarWalk) compress(name, fpath string, fi os.FileInfo) (bool, error) {
	isSym := fi.Mode()&os.ModeSymlink > 0
	link := ""
	if isSym {
		link, _ = os.Readlink(fpath)
	}

	hdr, err := tar.FileInfoHeader(fi, link)
	if err != nil {
		return false, err
	}
	hdr.Name = name

	tw := wft.tw
	err = tw.WriteHeader(hdr)
	if err != nil {
		return false, err
	}

	if isSym == false {
		fr, err := os.Open(fpath)
		if err != nil {
			return false, err
		}
		defer fr.Close()
		_, err = io.Copy(tw, fr)
		if err != nil {
			return false, err
		}
		tw.Flush()
	}

	return true, nil
}

type zipWalk struct {
	walkFileTree
	zw *zip.Writer
}

func (wft *zipWalk) compress(name, fpath string, fi os.FileInfo) (bool, error) {
	isSym := fi.Mode()&os.ModeSymlink > 0
	if isSym {
		// golang1.1 doesn't support embed symlink
		// what i miss something?
		return false, nil
	}

	hdr, err := zip.FileInfoHeader(fi)
	if err != nil {
		return false, err
	}
	hdr.Name = name

	zw := wft.zw
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return false, err
	}

	if isSym == false {
		fr, err := os.Open(fpath)
		if err != nil {
			return false, err
		}
		defer fr.Close()
		_, err = io.Copy(w, fr)
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func packDirectory(excludePrefix []string, excludeSuffix []string, includePath ...string) (err error) {
	fmt.Printf("exclude prefix: %s\n", strings.Join(excludePrefix, ":"))
	fmt.Printf("exclude suffix: %s\n", strings.Join(excludeSuffix, ":"))

	w, err := os.OpenFile(outputP, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	var wft walker

	if format == "zip" {
		walk := new(zipWalk)
		zw := zip.NewWriter(w)
		defer func() {
			zw.Close()
		}()
		walk.allfiles = make(map[string]bool)
		walk.zw = zw
		walk.wak = walk
		walk.excludePrefix = excludePrefix
		walk.excludeSuffix = excludeSuffix
		wft = walk
	} else {
		walk := new(tarWalk)
		cw := gzip.NewWriter(w)
		tw := tar.NewWriter(cw)

		defer func() {
			tw.Flush()
			cw.Flush()
			tw.Close()
			cw.Close()
		}()
		walk.allfiles = make(map[string]bool)
		walk.tw = tw
		walk.wak = walk
		walk.excludePrefix = excludePrefix
		walk.excludeSuffix = excludeSuffix
		wft = walk
	}

	for _, p := range includePath {
		err = wft.walkRoot(p)
		if err != nil {
			return
		}
	}

	return
}

func isBeegoProject(thePath string) bool {
	fh, _ := os.Open(thePath)
	fis, _ := fh.Readdir(-1)
	regex := regexp.MustCompile(`(?s)package main.*?import.*?\(.*?"github.com/astaxie/beego".*?\).*func main()`)
	for _, fi := range fis {
		if fi.IsDir() == false && strings.HasSuffix(fi.Name(), ".go") {
			data, err := ioutil.ReadFile(path.Join(thePath, fi.Name()))
			if err != nil {
				continue
			}
			if len(regex.Find(data)) > 0 {
				return true
			}
		}
	}
	return false
}

func packApp(cmd *Command, args []string) {
	curPath, _ := os.Getwd()
	thePath := ""

	nArgs := []string{}
	has := false
	for _, a := range args {
		if a != "" && a[0] == '-' {
			has = true
		}
		if has {
			nArgs = append(nArgs, a)
		}
	}
	cmdPack.Flag.Parse(nArgs)

	if path.IsAbs(appPath) == false {
		appPath = path.Join(curPath, appPath)
	}

	thePath, err := path.Abs(appPath)
	if err != nil {
		exitPrint(fmt.Sprintf("wrong app path: %s", thePath))
	}
	if stat, err := os.Stat(thePath); os.IsNotExist(err) || stat.IsDir() == false {
		exitPrint(fmt.Sprintf("not exist app path: %s", thePath))
	}

	if isBeegoProject(thePath) == false {
		exitPrint(fmt.Sprintf("not support non beego project"))
	}

	fmt.Printf("app path: %s\n", thePath)

	appName := path.Base(thePath)

	goos := runtime.GOOS
	if v, found := syscall.Getenv("GOOS"); found {
		goos = v
	}
	goarch := runtime.GOARCH
	if v, found := syscall.Getenv("GOARCH"); found {
		goarch = v
	}

	str := strconv.FormatInt(time.Now().UnixNano(), 10)[9:]

	gobin := path.Join(runtime.GOROOT(), "bin", "go")
	tmpdir := path.Join(os.TempDir(), "beePack-"+str)

	os.Mkdir(tmpdir, 0700)

	if build {
		fmt.Println("GOOS", goos, "GOARCH", goarch)
		fmt.Println("build", appName)

		os.Setenv("GOOS", goos)
		os.Setenv("GOARCH", goarch)

		binPath := path.Join(tmpdir, appName)
		if goos == "windows" {
			binPath += ".exe"
		}

		execmd := exec.Command(gobin, "build", "-o", binPath)
		execmd.Stdout = os.Stdout
		execmd.Stderr = os.Stderr
		execmd.Dir = thePath
		err = execmd.Run()
		if err != nil {
			exitPrint(err.Error())
		}

		fmt.Println("build success")
	}

	switch format {
	case "zip":
	default:
		format = "tar.gz"
	}

	outputN := appName + "." + format

	if outputP == "" || path.IsAbs(outputP) == false {
		outputP = path.Join(curPath, outputP)
	}

	if _, err := os.Stat(outputP); err != nil {
		err = os.MkdirAll(outputP, 0755)
		if err != nil {
			exitPrint(err.Error())
		}
	}

	outputP = path.Join(outputP, outputN)

	var exp, exs []string
	for _, p := range strings.Split(excludeP, ":") {
		if len(p) > 0 {
			exp = append(exp, p)
		}
	}
	for _, p := range strings.Split(excludeS, ":") {
		if len(p) > 0 {
			exs = append(exs, p)
		}
	}

	err = packDirectory(exp, exs, tmpdir, thePath)
	if err != nil {
		exitPrint(err.Error())
	}

	fmt.Printf("file write to `%s`\n", outputP)
}

// Go is a basic promise implementation: it wraps calls a function in a goroutine
// and returns a channel which will later return the function's return value.
func Go(f func() error) chan error {
	ch := make(chan error)
	go func() {
		ch <- f()
	}()
	return ch
}

// if os.env DEBUG set, debug is on
func Debugf(format string, a ...interface{}) {
	if os.Getenv("DEBUG") != "" {
		_, file, line, ok := runtime.Caller(1)
		if !ok {
			file = "<unknown>"
			line = -1
		} else {
			file = path.Base(file)
		}
		fmt.Fprintf(os.Stderr, fmt.Sprintf("[debug] %s:%d %s\n", file, line, format), a...)
	}
}

const (
	Gray = uint8(iota + 90)
	Red
	Green
	Yellow
	Blue
	Magenta
	//NRed      = uint8(31) // Normal
	EndColor = "\033[0m"
)

// colorLog colors log and print to stdout.
// Log format: [<level>] <content [path]> [ error ].
// Level: ERRO -> red; WARN -> Magenta; SUCC -> green; others -> default.
// Content: default; path: yellow; error -> red.
// Errors have to surrounded by "[ " and " ]"(space).
func colorLog(format string, a ...interface{}) {
	log := fmt.Sprintf(format, a...)
	if len(log) == 0 {
		return
	}

	if runtime.GOOS != "windows" {
		var clog string

		// Level.
		i := strings.Index(log, "]")
		if log[0] == '[' && i > -1 {
			clog += "[" + getColorLevel(log[1:i]) + "]"
		}

		log = log[i+1:]

		// Error.
		log = strings.Replace(log, "[ ", fmt.Sprintf("[\033[%dm", Red), -1)
		log = strings.Replace(log, " ]", EndColor+"]", -1)

		// Path.
		log = strings.Replace(log, "( ", fmt.Sprintf("(\033[%dm", Yellow), -1)
		log = strings.Replace(log, " )", EndColor+")", -1)

		// Highlights.
		log = strings.Replace(log, "# ", fmt.Sprintf("\033[%dm", Gray), -1)
		log = strings.Replace(log, " #", EndColor, -1)

		log = clog + log
	}

	fmt.Print(log)
}

// getColorLevel returns colored level string by given level.
func getColorLevel(level string) string {
	level = strings.ToUpper(level)
	switch level {
	case "TRAC":
		return fmt.Sprintf("\033[%dm%s\033[0m", Blue, level)
	case "ERRO":
		return fmt.Sprintf("\033[%dm%s\033[0m", Red, level)
	case "WARN":
		return fmt.Sprintf("\033[%dm%s\033[0m", Magenta, level)
	case "SUCC":
		return fmt.Sprintf("\033[%dm%s\033[0m", Green, level)
	default:
		return level
	}
}

var (
	cmd       *exec.Cmd
	state     sync.Mutex
	eventTime = make(map[string]int64)
)

func NewWatcher(paths []string) {
	fmt.Println("caoxiao", paths)
}

// getFileModTime retuens unix timestamp of `os.File.ModTime` by given path.
func getFileModTime(path string) int64 {
	f, err := os.Open(path)
	if err != nil {
		colorLog("[ERRO] Fail to open file[ %s ]", err)
		return time.Now().Unix()
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		colorLog("[ERRO] Fail to get file information[ %s ]", err)
		return time.Now().Unix()
	}

	return fi.ModTime().Unix()
}

func Autobuild() {
	state.Lock()
	defer state.Unlock()

	colorLog("[INFO] Start building...\n")
	path, _ := os.Getwd()
	os.Chdir(path)

	var err error
	// For applications use full import path like "github.com/.../.."
	// are able to use "go install" to reduce build time.
	if conf.GoInstall {
		icmd := exec.Command("go", "install")
		icmd.Stdout = os.Stdout
		icmd.Stderr = os.Stderr
		err = icmd.Run()
	}

	if err == nil {
		bcmd := exec.Command("go", "build")
		bcmd.Stdout = os.Stdout
		bcmd.Stderr = os.Stderr
		err = bcmd.Run()
	}

	if err != nil {
		colorLog("[ERRO] ============== Build failed ===================\n")
		return
	}
	colorLog("[SUCC] Build was successful\n")
	Restart(appname)
}

func Kill() {
	defer func() {
		if e := recover(); e != nil {
			fmt.Println("Kill -> ", e)
		}
	}()
	if cmd != nil {
		cmd.Process.Kill()
	}
}

func Restart(appname string) {
	Debugf("kill running process")
	Kill()
	go Start(appname)
}

func Start(appname string) {
	colorLog("[INFO] Restarting %s ...\n", appname)
	if strings.Index(appname, "./") == -1 {
		appname = "./" + appname
	}

	cmd = exec.Command(appname)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	go cmd.Run()
	//	started <- true
}

// checkTMPFile returns true if the event was for TMP files.
func checkTMPFile(name string) bool {
	if strings.HasSuffix(strings.ToLower(name), ".tmp") {
		return true
	}
	return false
}

// checkIsGoFile returns true if the name HasSuffix ".go".
func checkIsGoFile(name string) bool {
	if strings.HasSuffix(name, ".go") {
		return true
	}
	return false
}
