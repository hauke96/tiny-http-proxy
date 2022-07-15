package helper

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/kballard/go-shellquote"
)

var (
	//Debug variable to toggle debug output
	Debug bool
	//Verbose variable to toggle verbose output
	Verbose       bool
	Info          bool
	InfoTimestamp bool
	WarnExit      bool
	FatalExit     bool
	Quiet         bool
)

// ExecResult contains the exit code and output of an external command (e.g. git)
type ExecResult struct {
	ReturnCode int
	Output     string
}

// Debugf is a helper function for Debug logging if global variable Debug is set to true
func Debugf(s string) {
	if Debug != false {
		pc, _, _, _ := runtime.Caller(1)
		callingFunctionName := strings.Split(runtime.FuncForPC(pc).Name(), ".")[len(strings.Split(runtime.FuncForPC(pc).Name(), "."))-1]
		if strings.HasPrefix(callingFunctionName, "func") {
			// check for anonymous function names
			log.Print("Debug " + fmt.Sprint(s))
		} else {
			log.Print("Debug " + callingFunctionName + "(): " + fmt.Sprint(s))
		}
	}
}

// Verbosef is a helper function for Verbose logging if global variable Verbose is set to true
func Verbosef(s string) {
	if Debug != false || Verbose != false {
		log.Print(fmt.Sprint(s))
	}
}

// Infof is a helper function for Info logging if global variable Info is set to true
func Infof(s string) {
	if Debug != false || Verbose != false || Info != false {
		if InfoTimestamp {
			log.Print(fmt.Sprint(s))
		} else {
			color.Green(s)
		}
	}
}

// Warnf is a helper function for warning logging
func Warnf(s string) {
	pc, _, _, _ := runtime.Caller(1)
	callingFunctionName := strings.Split(runtime.FuncForPC(pc).Name(), ".")[len(strings.Split(runtime.FuncForPC(pc).Name(), "."))-1]
	color.Set(color.FgYellow)
	log.Print("WARN " + callingFunctionName + "(): " + fmt.Sprint(s))
	color.Unset()
	if WarnExit {
		os.Exit(1)
	}
}

// Fatalf is a helper function for fatal logging
func Fatalf(s string) {
	color.New(color.FgRed).Fprintln(os.Stderr, s)
	if FatalExit || WarnExit {
		os.Exit(1)
	}
}

// FileExists checks if the given file exists and returns a bool
func FileExists(file string) bool {
	//Debugf("checking for file existence " + file)
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return false
	}
	return true
}

// isDir checks if the given dir exists and returns a bool
func IsDir(dir string) bool {
	fi, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return false
	}
	if fi.Mode().IsDir() {
		return true
	}
	return false
}

// NormalizeDir removes from the given directory path multiple redundant slashes and adds a trailing slash
func NormalizeDir(dir string) string {
	if strings.Count(dir, "//") > 0 {
		dir = NormalizeDir(strings.Replace(dir, "//", "/", -1))
	} else {
		if !strings.HasSuffix(dir, "/") {
			dir = dir + "/"
		}
	}
	return dir
}

// checkDirAndCreate tests if the given directory exists and tries to create it
func CheckDirAndCreate(dir string, name string) (string, error) {
	if len(dir) != 0 {
		if !FileExists(dir) {
			//log.Printf("checkDirAndCreate(): trying to create dir '%s' as %s", dir, name){
			if err := os.MkdirAll(dir, 0777); err != nil {
				return "", errors.New("checkDirAndCreate(): failed to create directory: " + dir + " Error: " + err.Error())
			}
		} else {
			if !IsDir(dir) {
				return "", errors.New("checkDirAndCreate(): Error: " + dir + " exists, but is not a directory! Exiting!")
			}
		}
	}
	dir = NormalizeDir(dir)
	Debugf("Using as " + name + ": " + dir)
	return dir, nil
}

func CreateOrPurgeDir(dir string, callingFunction string) {
	if !FileExists(dir) {
		Debugf("Trying to create dir: " + dir + " called from " + callingFunction)
		os.MkdirAll(dir, 0777)
	} else {
		Debugf("Trying to remove: " + dir + " called from " + callingFunction)
		if err := os.RemoveAll(dir); err != nil {
			log.Print("createOrPurgeDir(): error: removing dir failed", err)
		}
		Debugf("Trying to create dir: " + dir + " called from " + callingFunction)
		os.MkdirAll(dir, 0777)
	}
}

func PurgeDir(dir string, callingFunction string) {
	if !FileExists(dir) {
		Debugf("Unnecessary to remove dir: " + dir + " it does not exist. Called from " + callingFunction)
	} else {
		Debugf("Trying to remove: " + dir + " called from " + callingFunction)
		if err := os.RemoveAll(dir); err != nil {
			log.Print("purgeDir(): os.RemoveAll() error: removing dir failed: ", err)
			if err = syscall.Unlink(dir); err != nil {
				log.Print("purgeDir(): syscall.Unlink() error: removing link failed: ", err)
			}
		}
	}
}

func ExecuteCommand(command string, timeout int, allowFail bool) ExecResult {
	Debugf("Executing " + command)
	parts := strings.SplitN(command, " ", 2)
	cmd := parts[0]
	cmdArgs := []string{}
	if len(parts) > 1 {
		args, err := shellquote.Split(parts[1])
		if err != nil {
			Debugf("err: " + fmt.Sprint(err))
		} else {
			cmdArgs = args
		}
	}

	before := time.Now()
	out, err := exec.Command(cmd, cmdArgs...).CombinedOutput()
	duration := time.Since(before).Seconds()
	er := ExecResult{0, string(out)}
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		er.ReturnCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}
	Debugf("Executing " + command + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
	if !allowFail && err != nil {
		Fatalf("executeCommand(): command failed: " + command + " " + err.Error() + "\nOutput: " + string(out))
	}
	if err != nil {
		er.ReturnCode = 1
		er.Output = fmt.Sprint(err) + " " + fmt.Sprint(string(out))
	}
	return er
}

// funcName return the function name as a string
func FuncName() string {
	pc, _, _, _ := runtime.Caller(1)
	completeFuncname := runtime.FuncForPC(pc).Name()
	return strings.Split(completeFuncname, ".")[len(strings.Split(completeFuncname, "."))-1]
}

func TimeTrack(start time.Time, name string) {
	duration := time.Since(start).Seconds()
	Debugf(name + "() took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
}

// getSha256sumFile return the SHA256 hash sum of the given file
func GetSha256sumFile(file string) string {
	// https://golang.org/pkg/crypto/sha256/#New
	f, err := os.Open(file)
	if err != nil {
		Fatalf("failed to open file " + file + " to calculate SHA256 sum. Error: " + err.Error())
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		Fatalf("failed to calculate SHA256 sum of file " + file + " Error: " + err.Error())
	}

	return string(h.Sum(nil))
}

// randSeq returns a fixed length random string to identify each request in the log
// http://stackoverflow.com/a/22892986/682847
func RandSeq() string {
	b := make([]rune, 8)
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	rand.Seed(time.Now().UTC().UnixNano())
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func WriteStructJSONFile(file string, v interface{}) {
	f, err := os.Create(file)
	if err != nil {
		Warnf("Could not write JSON file " + file + " " + err.Error())
	}
	defer f.Close()
	json, err := json.Marshal(v)
	if err != nil {
		Warnf("Could not encode JSON file " + file + " " + err.Error())
	}
	f.Write(json)
}

func KeysString(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func StringSliceContains(slice []string, element string) bool {
	for _, e := range slice {
		if e == element {
			return true
		}
	}
	return false
}

// GetRequestClientIp extract the client IP address from the request with support for X-Forwarded-For header when the request is originating from proxied networks
func GetRequestClientIp(request *http.Request, proxyNetworks []net.IPNet) (net.IP, error) {
	// RemoteAddr may contain a string like "[2001:db8::1]:12345", hence the following parsing
	// if the address belongs to a proxy network, the address from the X-Forwarded-For header is parsed and returned
	// an error is returned if parsing the X-Forwarded-For header failed (e.g. expected but not present)
	ipWithoutPortNumber := strings.TrimRight(request.RemoteAddr, "0123456789")
	ipPotentiallyWithBrackets := strings.TrimRight(ipWithoutPortNumber, ":")
	ipString := strings.Trim(ipPotentiallyWithBrackets, "[]")
	ip := net.ParseIP(ipString)
	if ip == nil {
		return nil, errors.New("failed to parse the client ip '" + ipString + "' from RemoteAddr " + request.RemoteAddr)
	}
	for _, proyxNetwork := range proxyNetworks {
		if proyxNetwork.Contains(ip) {
			xForwardedForHeaders := request.Header["X-Forwarded-For"]
			if len(xForwardedForHeaders) != 1 {
				return nil, errors.New("rejecting the request from " + request.RemoteAddr + " because it has " +
					strconv.Itoa(len(xForwardedForHeaders)) + " 'X-Forwarded-For' headers, but exactly one was expected")
			}
			ip = net.ParseIP(xForwardedForHeaders[0])
			if ip == nil {
				return nil, errors.New("failed to parse the X-Forwarded-For header '" + xForwardedForHeaders[0] +
					"' from RemoteAddr " + request.RemoteAddr)
			}
			return ip, nil
		}
	}
	return ip, nil
}

func ParseNetworks(networkStrings []string, contextMessage string) ([]net.IPNet, error) {
	var networks []net.IPNet
	for _, networkString := range networkStrings {
		_, network, err := net.ParseCIDR(networkString)
		if err != nil {
			m := contextMessage + ": failed to parse CIDR '" + networkString + "' " + err.Error()
			return nil, errors.New(m)
		}
		networks = append(networks, *network)
	}
	return networks, nil
}

func BoolPointer(b bool) *bool {
	return &b
}

func InBetween(i, min, max int) bool {
	if (i >= min) && (i <= max) {
		return true
	} else {
		return false
	}
}
