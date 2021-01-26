package display

import (
	"fmt"
	"github.com/logrusorgru/aurora"
	"strings"
	"time"

	"gopkg.in/auth0.v5/management"
)

func (r *Renderer) LogList(logs []*management.Log, noColor bool) {
	for _, l := range logs {
		logType := l.GetType()
		if !noColor {
			// colorize the event type field based on whether it's a success or failure
			if strings.HasPrefix(logType, "s") {
				logType = aurora.Green(logType).String()
			} else if strings.HasPrefix(logType, "f") {
				logType = aurora.BrightRed(logType).String()
			}
		}

		fmt.Fprintf(
			r.ResultWriter,
			"[%s] (%s) client_name=%q client_id=%q",
			l.Date.Format(time.RFC3339),
			logType,
			l.GetClientName(),
			l.GetClientID(),
		)

		// if userAgent is present in the log, then add it to the output
		reqMap, _ := l.Details["request"].(map[string]interface{})
		userAgent, _ := reqMap["userAgent"].(string)
		if userAgent != "" {
			fmt.Fprintf(
				r.ResultWriter,
				" user_agent=%q",
				userAgent,
			)
		}

		// if an error is present in the log, add it to the output
		errMap, _ := l.Details["error"].(map[string]interface{})
		errMsg, _ := errMap["message"].(string)
		errType, _ := errMap["type"].(string)
		if errType != "" || errMsg != "" {
			fmt.Fprintf(
				r.ResultWriter,
				" error_type=%q error_message=%q",
				errType,
				errMsg,
			)
		}

		fmt.Fprint(r.ResultWriter, "\n")
	}
}
