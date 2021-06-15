package gologger

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

//CustomLogger is heavily customisable logger which allows for complex file naming conventions and backups
type CustomLogger struct {
	Path string
	//Extention is the file extention; .txt, .json, .log
	Extention string
	//NamingConvention returns a string which will be used as the filename
	NamingConvention func() string

	//LineTerminator usually is "\n"
	LineTerminator string

	//ValueSeperator is inserted after each value in the data array of a write
	ValueSeperator string

	//ConventionUpdate is how long the logger should wait until it checks the current naming convention and if its time to change the file handle will be changed
	ConventionUpdate time.Duration

	//Callbacks

	//ConventionUpdated can be used to backup the old log file after the convention has change
	ConventionUpdated func(oldFile string, newFile string)

	queue chan ([]byte)

	close chan (struct{})
}

//NewCustomLogger creates a new custom logger
func NewCustomLogger(path string, extention string, bufferSize int) *CustomLogger {
	logger := &CustomLogger{
		Extention: extention,
		queue:     make(chan []byte, bufferSize),
		close:     make(chan struct{}),

		NamingConvention: func() string {
			return ""
		},

		//Never bother updating the convention
		ConventionUpdate: ((time.Hour * 24) * 7) * 2000,

		LineTerminator: "\n",
		Path:           path,
		ValueSeperator: " ",
	}

	return logger
}

//Write will add the data to the queue then write to disk
func (l *CustomLogger) Write(data ...interface{}) {
	payload := l.convertInput(data)
	l.queue <- payload
}

func (l *CustomLogger) convertInput(input []interface{}) []byte {
	var payload []byte
	for i, x := range input {

		if i != 0 {
			payload = append(payload, []byte(l.ValueSeperator)...)
		}

		switch x.(type) {
		case []byte:
			payload = append(payload, x.([]byte)...)
		default:
			payload = append(payload, fmt.Sprint(x)...)
		}
	}

	return payload
}

//WritePrint will write the data to the file but also print it to the screen
func (l *CustomLogger) WritePrint(data ...interface{}) {
	payload := l.convertInput(data)
	println(string(payload))
	l.queue <- payload
}

func (l *CustomLogger) getFileName() string {
	return l.Path + l.NamingConvention() + l.Extention
}

func (l *CustomLogger) getFileHandle() (*os.File, string, error) {
	fileName := l.getFileName()

	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, "", err
	}

	return file, fileName, nil
}

//Service will start the logging service
func (l *CustomLogger) Service() error {

	handle, fileName, err := l.getFileHandle()
	if err != nil {
		return err
	}

	var newFileName string
	t := time.NewTicker(l.ConventionUpdate)
	defer t.Stop()
	for {
		select {

		case <-t.C:

			handle, newFileName, err = l.getFileHandle()
			if err != nil {
				return err
			}

			fileName = newFileName

			if l.ConventionUpdated != nil {
				//Run on another goroutine to prevent the service worker from hanging
				go l.ConventionUpdated(fileName, newFileName)
			}

			break
		case data := <-l.queue:
			handle.Write(append(data, l.LineTerminator...))
			break
		case <-l.close:
			return handle.Close()
		}
	}
}

//Close will shutdown the service worker
func (l *CustomLogger) Close() {
	l.close <- struct{}{}
}

//WriteJSON will take in a object and serialise it as JSON
func (l *CustomLogger) WriteJSON(object interface{}) {

	b, err := json.Marshal(object)
	if err != nil {
		return
	}

	l.queue <- b
}

//QueueLength returns the length of the queue
func (l *CustomLogger) QueueLength() int {
	return len(l.queue)
}
