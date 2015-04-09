package pidfile

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

type PidFile struct {
	path string
}

// 检查是不是有一个docker daemon已经在运行了
func checkPidFileAlreadyExists(path string) error {
	// 读取pidfile中的内容
	if pidString, err := ioutil.ReadFile(path); err == nil {
		// 将内容转换为一个整数
		if pid, err := strconv.Atoi(string(pidString)); err == nil {
			// 如果这个整数代表的进程正在执行,表示已经有一个docker daemon在运行了
			if _, err := os.Stat(filepath.Join("/proc", string(pid))); err == nil {
				return fmt.Errorf("pid file found, ensure docker is not running or delete %s", path)
			}
		}
	}
	return nil
}

// 新建一个docker daemon运行时的pid文件, path为文件的路径
func New(path string) (file *PidFile, err error) {
	// 保证系统中当前没有docker daemon在运行
	if err := checkPidFileAlreadyExists(path); err != nil {
		return nil, err
	}

	file = &PidFile{path: path}
	err = ioutil.WriteFile(path, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)

	return file, err
}

func (file PidFile) Remove() error {
	if err := os.Remove(file.path); err != nil {
		log.Printf("Error removing %s: %s", file.path, err)
		return err
	}
	return nil
}
