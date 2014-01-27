/**
从config.go中 看过来
**/
package process

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

// 保存pid
func SavePidTo(pidFile string) error {
	pidPath := filepath.Dir(pidFile)
	if err := os.MkdirAll(pidPath, 0777); err != nil {
		return err
	}
	return ioutil.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0777)
}

// 获得可执行程序所在目录
func ExecutableDir() (string, error) {
	pathAbs, err := filepath.Abs(os.Args[0])
	fmt.Println("os.Args[0]:::::", os.Args[0])
	fmt.Println("pathAbs:::::", pathAbs)
	if err != nil {
		return "", err
	}
	return filepath.Dir(pathAbs), nil
}
