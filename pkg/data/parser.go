package data

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type MessageStyle string

const (
	MessageStyleMuted   MessageStyle = "muted"
	MessageStyleInfo    MessageStyle = "info"
	MessageStyleSuccess MessageStyle = "success"
	MessageStyleWarning MessageStyle = "warning"
)

var (
	invokeMatcher   = regexp.MustCompile(`Program (\w*) invoke \[(\d)\]`)
	consumedMatcher = regexp.MustCompile(`Program \w* consumed (\d*) (.*)`)
)

type LogMessage struct {
	Text   string
	Prefix string
	Style  MessageStyle
}

type InstructionLogs struct {
	Program      string
	Logs         []LogMessage
	ComputeUnits uint
	Truncated    bool
	Failed       bool
}

func prefixBuilder(depth int) string {
	return ">"
}

func ParseLogs(logs []string) []InstructionLogs {
	var depth int
	instLogs := []InstructionLogs{}

	for _, log := range logs {
		if strings.HasPrefix(log, "Program log:") {
			instLogs[len(instLogs)-1].Logs = append(instLogs[len(instLogs)-1].Logs, LogMessage{
				Prefix: prefixBuilder(depth),
				Style:  MessageStyleMuted,
				Text:   log,
			})
		} else if strings.HasPrefix(log, "Log truncated") {
			instLogs[len(instLogs)-1].Truncated = true
		} else {
			matches := invokeMatcher.FindStringSubmatch(log)

			if len(matches) > 0 {
				if depth == 0 {
					instLogs = append(instLogs, InstructionLogs{
						ComputeUnits: 0,
						Failed:       false,
						Program:      matches[1],
						Logs:         []LogMessage{},
						Truncated:    false,
					})
				} else {
					instLogs[len(instLogs)-1].Logs = append(instLogs[len(instLogs)-1].Logs, LogMessage{
						Prefix: prefixBuilder(depth),
						Style:  MessageStyleInfo,
						Text:   fmt.Sprintf("Program invoked: %s", matches[1]),
					})
				}

				depth++
			} else if strings.Contains(log, "success") {
				instLogs[len(instLogs)-1].Logs = append(instLogs[len(instLogs)-1].Logs, LogMessage{
					Prefix: prefixBuilder(depth),
					Style:  MessageStyleSuccess,
					Text:   "Program returned success",
				})

				depth--
			} else if strings.Contains(log, "failed") {
				instLogs[len(instLogs)-1].Failed = true

				idx := strings.Index(log, ": ") + 2
				currText := fmt.Sprintf(`Program returned error: "%s"`, log[idx:])

				// failed to verify log of previous program so reset depth and print full log
				if strings.HasPrefix(log, "failed") {
					depth++

					currText = strings.ToTitle(log)
				}

				instLogs[len(instLogs)-1].Logs = append(instLogs[len(instLogs)-1].Logs, LogMessage{
					Prefix: prefixBuilder(depth),
					Style:  MessageStyleWarning,
					Text:   currText,
				})

				depth--
			} else {
				if depth == 0 {
					instLogs = append(instLogs, InstructionLogs{
						ComputeUnits: 0,
						Failed:       false,
						Program:      "",
						Logs:         []LogMessage{},
						Truncated:    false,
					})
				}

				matches := consumedMatcher.FindStringSubmatch(log)
				if len(matches) == 3 {
					if depth == 1 {
						if val, err := strconv.Atoi(matches[1]); err == nil {
							instLogs[len(instLogs)-1].ComputeUnits = uint(val)
						}
					}

					log = fmt.Sprintf("Program consumed: %s %s", matches[1], matches[2])
				}

				// native program logs don't start with "Program log:"
				instLogs[len(instLogs)-1].Logs = append(instLogs[len(instLogs)-1].Logs, LogMessage{
					Prefix: prefixBuilder(depth),
					Style:  MessageStyleMuted,
					Text:   log,
				})
			}
		}
	}

	return instLogs
}
