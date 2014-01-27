/*
2013.10.10阅读
发现引用process包,process包已读
读取config.json 并把文件内容放在内存中
*/
package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"process"
)

// 项目根目录
var ROOT string

var Config map[string]string

func init() {
	binDir, err := process.ExecutableDir()
	if err != nil {
		panic(err)
	}
	fmt.Println("binDir::::", binDir)
	ROOT = path.Dir(binDir)
	fmt.Println("root:::", ROOT)
	// Load配置文件
	configFile := ROOT + "/conf/config.json"
	content, err := ioutil.ReadFile(configFile)
	if err != nil {
		panic(err)
	}
	Config = make(map[string]string)
	err = json.Unmarshal(content, &Config)
	if err != nil {
		panic(err)
	}
}
