/*
 * Minio Cloud Storage, (C) 2015, 2016, 2017 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/minio/mc/pkg/console"
	"github.com/minio/minio/pkg/errors"
)

var log = NewLogger()
var trimStrings []string

// Level type
type Level int8

// Enumerated level types
const (
	Error Level = iota + 1
	Fatal
)

func (level Level) String() string {
	var lvlStr string
	switch level {
	case Error:
		lvlStr = "ERROR"
	case Fatal:
		lvlStr = "FATAL"
	}
	return lvlStr
}

type logEntry struct {
	Level   string   `json:"level"`
	Message string   `json:"message"`
	Time    string   `json:"time"`
	Cause   string   `json:"cause"`
	Trace   []string `json:"trace"`
}

// Logger - for console messages
type Logger struct {
	quiet bool
	json  bool
}

// NewLogger - to create a new Logger object
func NewLogger() *Logger {
	return &Logger{}
}

// EnableQuiet - turns quiet option on.
func (log *Logger) EnableQuiet() {
	log.quiet = true
}

// EnableJSON - outputs logs in json format.
func (log *Logger) EnableJSON() {
	log.json = true
	log.quiet = true
}

// Println - wrapper to console.Println() with quiet flag.
func (log *Logger) Println(args ...interface{}) {
	if !log.quiet {
		console.Println(args...)
	}
}

// Printf - wrapper to console.Printf() with quiet flag.
func (log *Logger) Printf(format string, args ...interface{}) {
	if !log.quiet {
		console.Printf(format, args...)
	}
}

func init() {
	var goPathList []string
	// Add all possible GOPATH paths into trimStrings
	// Split GOPATH depending on the OS type
	if runtime.GOOS == "windows" {
		goPathList = strings.Split(GOPATH, ";")
	} else {
		// All other types of OSs
		goPathList = strings.Split(GOPATH, ":")
	}

	// Add trim string "{GOROOT}/src/" into trimStrings
	trimStrings = []string{filepath.Join(runtime.GOROOT(), "src") + string(filepath.Separator)}

	// Add all possible path from GOPATH=path1:path2...:pathN
	// as "{path#}/src/" into trimStrings
	for _, goPathString := range goPathList {
		trimStrings = append(trimStrings, filepath.Join(goPathString, "src")+string(filepath.Separator))
	}
	// Add "github.com/minio/minio" as the last to cover
	// paths like "{GOROOT}/src/github.com/minio/minio"
	// and "{GOPATH}/src/github.com/minio/minio"
	trimStrings = append(trimStrings, filepath.Join("github.com", "minio", "minio")+string(filepath.Separator))
}

func trimTrace(f string) string {
	for _, trimString := range trimStrings {
		f = strings.TrimPrefix(filepath.ToSlash(f), filepath.ToSlash(trimString))
	}
	return filepath.FromSlash(f)
}

// getTrace method - creates and returns stack trace
func getTrace(traceLevel int) []string {
	var trace []string
	pc, file, lineNumber, ok := runtime.Caller(traceLevel)

	for ok {
		// Clean up the common prefixes
		file = trimTrace(file)
		// Get the function name
		_, funcName := filepath.Split(runtime.FuncForPC(pc).Name())
		// Skip duplicate traces that start with file name, "<autogenerated>"
		// and also skip traces with function name that starts with "runtime."
		if !strings.HasPrefix(file, "<autogenerated>") &&
			!strings.HasPrefix(funcName, "runtime.") {
			// Form and append a line of stack trace into a
			// collection, 'trace', to build full stack trace
			trace = append(trace, fmt.Sprintf("%v:%v:%v()", file, lineNumber, funcName))
		}
		traceLevel++
		// Read stack trace information from PC
		pc, file, lineNumber, ok = runtime.Caller(traceLevel)
	}
	return trace
}

func logIf(level Level, err error, msg string,
	data ...interface{}) {

	isErrIgnored := func(err error) (ok bool) {
		err = errors.Cause(err)
		switch err.(type) {
		case BucketNotFound, BucketNotEmpty, BucketExists:
			ok = true
		case ObjectNotFound, ObjectExistsAsDirectory:
			ok = true
		case BucketPolicyNotFound, InvalidUploadID:
			ok = true
		}
		return ok
	}

	if err == nil || isErrIgnored(err) {
		return
	}
	cause := strings.Title(err.Error())
	// Get full stack trace
	trace := getTrace(3)
	// Get time
	timeOfError := UTCNow().Format(time.RFC3339Nano)
	// Output the formatted log message at console
	var output string
	message := fmt.Sprintf(msg, data...)
	if log.json {
		logJSON, err := json.Marshal(&logEntry{
			Level:   level.String(),
			Message: message,
			Time:    timeOfError,
			Cause:   cause,
			Trace:   trace,
		})
		if err != nil {
			panic("json marshal of logEntry failed: " + err.Error())
		}
		output = string(logJSON)
	} else {
		// Add a sequence number and formatting for each stack trace
		// No formatting is required for the first entry
		trace[0] = "1: " + trace[0]
		for i, element := range trace[1:] {
			trace[i+1] = fmt.Sprintf("%8v: %s", i+2, element)
		}
		errMsg := fmt.Sprintf("[%s] [%s] %s (%s)",
			timeOfError, level.String(), message, cause)

		output = fmt.Sprintf("\nTrace: %s\n%s",
			strings.Join(trace, "\n"),
			colorRed(colorBold(errMsg)))
	}
	fmt.Println(output)

	if level == Fatal {
		os.Exit(1)
	}
}

func errorIf(err error, msg string, data ...interface{}) {
	logIf(Error, err, msg, data...)
}

func fatalIf(err error, msg string, data ...interface{}) {
	logIf(Fatal, err, msg, data...)
}
