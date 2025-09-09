package logger

import (
	"log"
	"os"
)

var Log *log.Logger

func Init(logFilePath string) error {
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}

	Log = log.New(file, "", log.LstdFlags)
	Log.Println("Logger initialized.")
	return nil
}
