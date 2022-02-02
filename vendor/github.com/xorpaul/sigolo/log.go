package sigolo

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"time"

	"github.com/gookit/color"
)

type Level int

const (
	LOG_PLAIN Level = iota
	LOG_DEBUG
	LOG_INFO
	LOG_WARN
	LOG_ERROR
	LOG_FATAL
)

var (
	LogLevel      Level  = LOG_INFO
	DateFormat    string = "2006-01-02 15:04:05.000"
	SkipTimestamp bool   = false
	EnableColors  bool   = false

	FormatFunctions map[Level]func(*os.File, string, string, int, string, string) = map[Level]func(*os.File, string, string, int, string, string){
		LOG_PLAIN: LogPlain,
		LOG_DEBUG: LogDefault,
		LOG_INFO:  LogDefault,
		LOG_WARN:  LogDefault,
		LOG_ERROR: LogDefault,
		LOG_FATAL: LogDefault,
	}

	// The current maximum length printed for caller information. This is updated each time something gets printed
	CallerColumnWidth = 0

	LevelStrings map[Level]string = map[Level]string{
		LOG_PLAIN: "",
		LOG_DEBUG: "[DEBUG]",
		LOG_INFO:  "[INFO] ",
		LOG_WARN:  "[WARN]",
		LOG_ERROR: "[ERROR]",
		LOG_FATAL: "[FATAL]",
	}

	LevelOutputs map[Level]*os.File = map[Level]*os.File{
		LOG_PLAIN: os.Stdout,
		LOG_DEBUG: os.Stdout,
		LOG_INFO:  os.Stdout,
		LOG_WARN:  os.Stderr,
		LOG_ERROR: os.Stderr,
		LOG_FATAL: os.Stderr,
	}
)

func Plain(format string, a ...interface{}) {
	if LogLevel > LOG_PLAIN {
		return
	}
	log(LOG_PLAIN, 3, fmt.Sprintf(format, a...))
}

// Plainb is equal to Plain(...) but can go back in the stack and can therefore show function positions from previous functions.
func Plainb(framesBackward int, format string, a ...interface{}) {
	if LogLevel > LOG_PLAIN {
		return
	}
	log(LOG_PLAIN, 3+framesBackward, fmt.Sprintf(format, a...))
}

func Info(format string, a ...interface{}) {
	if LogLevel > LOG_INFO {
		return
	}
	if EnableColors {
		color.Set(color.FgGreen, color.OpBold)
	}
	log(LOG_INFO, 3, fmt.Sprintf(format, a...))
}

// Infob is equal to Info(...) but can go back in the stack and can therefore show function positions from previous functions.
func Infob(framesBackward int, format string, a ...interface{}) {
	if LogLevel > LOG_INFO {
		return
	}
	if EnableColors {
		color.Set(color.FgGreen, color.OpBold)
	}
	log(LOG_INFO, 3+framesBackward, fmt.Sprintf(format, a...))
}

func Debug(format string, a ...interface{}) {
	if LogLevel > LOG_DEBUG {
		return
	}
	log(LOG_DEBUG, 3, fmt.Sprintf(format, a...))
}

// Debugb is equal to Debug(...) but can go back in the stack and can therefore show function positions from previous functions.
func Debugb(framesBackward int, format string, a ...interface{}) {
	if LogLevel > LOG_DEBUG {
		return
	}
	log(LOG_DEBUG, 3+framesBackward, fmt.Sprintf(format, a...))
}

func Warn(format string, a ...interface{}) {
	if LogLevel > LOG_WARN {
		return
	}
	if EnableColors {
		color.Set(color.FgYellow, color.OpBold)
	}
	log(LOG_WARN, 3, fmt.Sprintf(format, a...))
}

// Warnb is equal to Warn(...) but can go back in the stack and can therefore show function positions from previous functions.
func Warnb(framesBackward int, format string, a ...interface{}) {
	if LogLevel > LOG_WARN {
		return
	}
	if EnableColors {
		color.Set(color.FgYellow, color.OpBold)
	}
	log(LOG_WARN, 3+framesBackward, fmt.Sprintf(format, a...))
}

func Error(format string, a ...interface{}) {
	if LogLevel > LOG_ERROR {
		return
	}
	if EnableColors {
		color.Set(color.FgRed, color.OpBold)
	}
	log(LOG_ERROR, 3, fmt.Sprintf(format, a...))
}

// Errorb is equal to Error(...) but can go back in the stack and can therefore show function positions from previous functions.
func Errorb(framesBackward int, format string, a ...interface{}) {
	if LogLevel > LOG_ERROR {
		return
	}
	if EnableColors {
		color.Set(color.FgRed, color.OpBold)
	}
	log(LOG_ERROR, 3+framesBackward, fmt.Sprintf(format, a...))
}

// Stack tries to print the stack trace of the given error using the  %+v  format string. When using the
// https://github.com/pkg/errors package, this will print a full stack trace of the error. If normal errors are used,
// this function will just print the error.
func Stack(err error) {
	if LogLevel > LOG_ERROR {
		return
	}
	// Directly call "log" to avoid extra function call
	log(LOG_ERROR, 3, fmt.Sprintf("%+v", err))
}

// Stackb is equal to Stack(...) but can go back in the stack and can therefore show function positions from previous functions.
func Stackb(framesBackward int, err error) {
	if LogLevel > LOG_ERROR {
		return
	}
	// Directly call "log" to avoid extra function call
	log(LOG_ERROR, 3+framesBackward, fmt.Sprintf("%+v", err))
}

func internalError(format string, a ...interface{}) {
	internalLog(LOG_ERROR, fmt.Sprintf(format, a...))
}

func Fatal(format string, a ...interface{}) {
	if EnableColors {
		color.Set(color.FgRed, color.OpBold)
	}
	log(LOG_FATAL, 3, fmt.Sprintf(format, a...))
	os.Exit(1)
}

func internalFatal(format string, a ...interface{}) {
	internalLog(LOG_FATAL, fmt.Sprintf(format, a...))
	os.Exit(1)
}

// FatalCheckf checks if the error exists (!= nil). If so, it'll print the error
// message and fatals with the given format message.
func FatalCheckf(err error, format string, a ...interface{}) {
	if err != nil {
		internalError(err.Error())
		if a != nil {
			internalFatal(format, a...)
		} else {
			internalFatal(format)
		}
	}
}

// FatalCheck checks if the error exists (!= nil). If so, it'll fatal with the error message.
func FatalCheck(err error) {
	if err != nil {
		internalFatal(err.Error())
	}
}

func log(level Level, framesBackward int, message string) {
	// A bit hacky: We know here that the stack contains two calls from inside
	// this file. The third frame comes from the file that initially called a
	// function in this file (e.g. Info())
	caller := getCallerDetails(framesBackward)

	updateCallerColumnWidth(caller)

	FormatFunctions[level](LevelOutputs[level], time.Now().Format(DateFormat), LevelStrings[level], CallerColumnWidth, caller, message)
}

func internalLog(level Level, message string) {
	// A bit hacky: We know here that the stack contains three calls from inside
	// this file. The third frame comes from the file that initially called a
	// function in this file (e.g. Info())
	caller := getCallerDetails(4)

	updateCallerColumnWidth(caller)

	FormatFunctions[level](LevelOutputs[level], time.Now().Format(DateFormat), LevelStrings[level], CallerColumnWidth, caller, message)
}

func updateCallerColumnWidth(caller string) {
	if len(caller) > CallerColumnWidth {
		CallerColumnWidth = len(caller)
	}
}

func getCallerDetails(framesBackwards int) string {
	name := "???"
	line := -1
	ok := false

	_, name, line, ok = runtime.Caller(framesBackwards)

	if ok {
		name = path.Base(name)
	}

	caller := fmt.Sprintf("%s:%d", name, line)

	return caller
}

func LogDefault(writer *os.File, time, level string, maxLength int, caller, message string) {
	if SkipTimestamp {
		fmt.Fprintf(writer, "%s %-*s | %s\n", level, maxLength, caller, message)
	} else {
		fmt.Fprintf(writer, "%s %s %-*s | %s\n", time, level, maxLength, caller, message)
	}
	color.Reset()
}

func LogPlain(writer *os.File, time, level string, maxLength int, caller, message string) {
	fmt.Fprintf(writer, "%s\n", message)
}
