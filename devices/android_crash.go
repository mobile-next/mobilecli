package devices

import (
	"fmt"
	"regexp"
	"strings"
)

// logcat -v year line format: YYYY-MM-DD HH:MM:SS.mmm  PID  TID LEVEL TAG     : message
var logcatLineRegex = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})\s+(\d{2}:\d{2}:\d{2}\.\d{3})\s+(\d+)\s+\d+\s+(\w)\s+(\S+)\s*: (.*)$`)

type logcatLine struct {
	Date    string // YYYY-MM-DD
	Time    string // HH:MM:SS.mmm
	PID     string
	Level   string
	Tag     string
	Message string
	Raw     string
}

func parseLogcatLine(line string) *logcatLine {
	m := logcatLineRegex.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	return &logcatLine{
		Date:    m[1],
		Time:    m[2],
		PID:     m[3],
		Level:   m[4],
		Tag:     strings.TrimSpace(m[5]),
		Message: m[6],
		Raw:     line,
	}
}

func makeAndroidCrashID(date string, time string, pid string) string {
	return fmt.Sprintf("%s_%s_%s", date, time, pid)
}

type parsedCrash struct {
	report   CrashReport
	startIdx int
	endIdx   int
}

func parseAllCrashes(log string) []parsedCrash {
	lines := strings.Split(log, "\n")
	var crashes []parsedCrash

	i := 0
	for i < len(lines) {
		parsed := parseLogcatLine(lines[i])
		if parsed == nil {
			i++
			continue
		}

		var crash *CrashReport
		var end int

		if isNativeCrashStart(parsed) {
			crash, end = parseNativeCrash(lines, i)
		} else if isJavaCrashStart(parsed) {
			crash, end = parseJavaCrash(lines, i)
		} else {
			i++
			continue
		}

		if crash != nil {
			crashes = append(crashes, parsedCrash{report: *crash, startIdx: i, endIdx: end})
		}
		i = end
	}

	return crashes
}

// ParseAndroidCrashLog parses the output of `adb logcat -b crash -d -v year`
// and returns a list of crash reports
func ParseAndroidCrashLog(log string) []CrashReport {
	parsed := parseAllCrashes(log)
	crashes := make([]CrashReport, len(parsed))
	for i, p := range parsed {
		crashes[i] = p.report
	}
	return crashes
}

func isNativeCrashStart(l *logcatLine) bool {
	return l.Level == "F" && l.Tag == "DEBUG" && strings.Contains(l.Message, "*** ***")
}

func isJavaCrashStart(l *logcatLine) bool {
	return l.Level == "E" && l.Tag == "AndroidRuntime" && strings.HasPrefix(l.Message, "FATAL EXCEPTION:")
}

var nativePidRegex = regexp.MustCompile(`pid:\s*(\d+)`)
var nativeCmdlineRegex = regexp.MustCompile(`Cmdline:\s*(.+)`)

func parseNativeCrash(lines []string, start int) (*CrashReport, int) {
	startLine := parseLogcatLine(lines[start])
	if startLine == nil {
		return nil, start + 1
	}

	debugPID := startLine.PID
	processName := ""
	crashPID := ""

	i := start + 1
	for i < len(lines) {
		parsed := parseLogcatLine(lines[i])
		if parsed == nil {
			break
		}
		// native tombstone lines all come from the same debuggerd PID with tag DEBUG
		if parsed.PID != debugPID || parsed.Tag != "DEBUG" {
			break
		}

		if m := nativeCmdlineRegex.FindStringSubmatch(parsed.Message); m != nil {
			processName = strings.TrimSpace(m[1])
		}
		if m := nativePidRegex.FindStringSubmatch(parsed.Message); m != nil && crashPID == "" {
			crashPID = m[1]
		}

		i++
	}

	if crashPID == "" {
		crashPID = debugPID
	}

	return &CrashReport{
		ProcessName: processName,
		Timestamp:   startLine.Date + " " + startLine.Time,
		ID:          makeAndroidCrashID(startLine.Date, startLine.Time, crashPID),
	}, i
}

func parseJavaCrash(lines []string, start int) (*CrashReport, int) {
	startLine := parseLogcatLine(lines[start])
	if startLine == nil {
		return nil, start + 1
	}

	pid := startLine.PID
	processName := ""

	// scan for Process: or PID: line, and collect all AndroidRuntime lines with same pid
	i := start + 1
	for i < len(lines) {
		parsed := parseLogcatLine(lines[i])
		if parsed == nil {
			break
		}
		if parsed.Tag != "AndroidRuntime" || parsed.PID != pid {
			break
		}

		if strings.HasPrefix(parsed.Message, "Process: ") {
			// "Process: com.example.app, PID: 12345"
			parts := strings.SplitN(parsed.Message, ",", 2)
			processName = strings.TrimPrefix(parts[0], "Process: ")
			processName = strings.TrimSpace(processName)
		}

		i++
	}

	// if no Process: line, try to extract from the first stack frame
	if processName == "" {
		processName = extractProcessFromStack(lines, start, pid)
	}

	return &CrashReport{
		ProcessName: processName,
		Timestamp:   startLine.Date + " " + startLine.Time,
		ID:          makeAndroidCrashID(startLine.Date, startLine.Time, pid),
	}, i
}

func extractProcessFromStack(lines []string, start int, pid string) string {
	for i := start + 1; i < len(lines); i++ {
		parsed := parseLogcatLine(lines[i])
		if parsed == nil || parsed.PID != pid || parsed.Tag != "AndroidRuntime" {
			break
		}
		if name := parseStackFrameOwner(parsed.Message); name != "" {
			return name
		}
	}
	return ""
}

// parseStackFrameOwner extracts the fully qualified class name from a stack
// frame like "at com.example.Foo$1.onClick(Foo.java:42)". it finds the last
// dot before the opening paren to split class from method.
func parseStackFrameOwner(message string) string {
	msg := strings.TrimSpace(message)
	if !strings.HasPrefix(msg, "at ") {
		return ""
	}
	fqn := msg[3:]

	parenIdx := strings.Index(fqn, "(")
	if parenIdx < 0 {
		return ""
	}
	fqn = fqn[:parenIdx]

	dotIdx := strings.LastIndex(fqn, ".")
	if dotIdx < 1 {
		return ""
	}
	return fqn[:dotIdx]
}

// ExtractAndroidCrash finds a specific crash by ID from the logcat output
// and returns the full crash text
func ExtractAndroidCrash(log string, id string) (string, error) {
	lines := strings.Split(log, "\n")

	for _, c := range parseAllCrashes(log) {
		if c.report.ID == id {
			return strings.Join(lines[c.startIdx:c.endIdx], "\n"), nil
		}
	}

	return "", fmt.Errorf("crash %s not found", id)
}
