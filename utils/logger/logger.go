package logger

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const (
	DEFAULT_PATH string = "storage/logs/log.log" // 日志默认保存路径
)

type Logger struct {}

func (l *Logger) record(msg interface{}, target string) error {
	entension := filepath.Ext(target)
	if entension == "" {
		return errors.New(fmt.Sprintf("File name cannot be empty %s", target))
	}

	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	file, err := os.OpenFile(target, os.O_APPEND|os.O_CREATE, 666)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not create fileer %s", target))
	}
	defer file.Close()

	logger := log.New(file, "", log.LstdFlags)
	logger.Println(msg)
	return nil
}

func makepath(args ...interface{}) string {
	target := DEFAULT_PATH
	if len(args) > 0 {
		target = args[0].(string)
	}
	return target
}

func Println(msg interface{}, args ...interface{}) {
	Info(msg, args...)
}

func Info(msg interface{}, args ...interface{}) {
	if err := new(Logger).record(msg, makepath(args...)); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}