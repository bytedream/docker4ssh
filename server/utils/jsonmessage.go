// adopted from https://github.com/docker/cli/blob/a32cd16160f1b41c1c4ae7bee4dac929d1484e59/vendor/github.com/docker/docker/pkg/jsonmessage/jsonmessage.go

package utils

import (
	"docker4ssh/terminal"
	"encoding/json"
	"fmt"
	"github.com/docker/go-units"
	"github.com/morikuni/aec"
	"io"
	"strings"
	"time"
)

// RFC3339NanoFixed is time.RFC3339Nano with nanoseconds padded using zeros to
// ensure the formatted time is always the same number of characters.
const RFC3339NanoFixed = "2006-01-02T15:04:05.000000000Z07:00"

// JSONError wraps a concrete Code and Message, `Code` is
// an integer error code, `Message` is the error message.
type JSONError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (e *JSONError) Error() string {
	return e.Message
}

// JSONProgress describes a progress. terminalFd is the fd of the current terminal,
// Start is the initial value for the operation. Current is the current status and
// value of the progress made towards Total. Total is the end value describing when
// we made 100% progress for an operation.
type JSONProgress struct {
	Terminal *terminal.Terminal
	Current  int64 `json:"current,omitempty"`
	Total    int64 `json:"total,omitempty"`
	Start    int64 `json:"start,omitempty"`
	// If true, don't show xB/yB
	HideCounts bool   `json:"hidecounts,omitempty"`
	Units      string `json:"units,omitempty"`
}

func (p *JSONProgress) String() string {
	var (
		width       = p.Terminal.Width
		pbBox       string
		numbersBox  string
		timeLeftBox string
	)
	if p.Current <= 0 && p.Total <= 0 {
		return ""
	}
	if p.Total <= 0 {
		switch p.Units {
		case "":
			current := units.HumanSize(float64(p.Current))
			return fmt.Sprintf("%8v", current)
		default:
			return fmt.Sprintf("%d %s", p.Current, p.Units)
		}
	}

	percentage := int(float64(p.Current)/float64(p.Total)*100) / 2
	if percentage > 50 {
		percentage = 50
	}
	if width > 110 {
		// this number can't be negative gh#7136
		numSpaces := 0
		if 50-percentage > 0 {
			numSpaces = 50 - percentage
		}
		pbBox = fmt.Sprintf("[%s>%s] ", strings.Repeat("=", percentage), strings.Repeat(" ", numSpaces))
	}

	switch {
	case p.HideCounts:
	case p.Units == "": // no units, use bytes
		current := units.HumanSize(float64(p.Current))
		total := units.HumanSize(float64(p.Total))

		numbersBox = fmt.Sprintf("%8v/%v", current, total)

		if p.Current > p.Total {
			// remove total display if the reported current is wonky.
			numbersBox = fmt.Sprintf("%8v", current)
		}
	default:
		numbersBox = fmt.Sprintf("%d/%d %s", p.Current, p.Total, p.Units)

		if p.Current > p.Total {
			// remove total display if the reported current is wonky.
			numbersBox = fmt.Sprintf("%d %s", p.Current, p.Units)
		}
	}

	if p.Current > 0 && p.Start > 0 && percentage < 50 {
		fromStart := time.Now().Sub(time.Unix(p.Start, 0))
		perEntry := fromStart / time.Duration(p.Current)
		left := time.Duration(p.Total-p.Current) * perEntry
		left = (left / time.Second) * time.Second

		if width > 50 {
			timeLeftBox = " " + left.String()
		}
	}
	return pbBox + numbersBox + timeLeftBox
}

// JSONMessage defines a message struct. It describes
// the created time, where it from, status, ID of the
// message. It's used for docker events.
type JSONMessage struct {
	Stream          string        `json:"stream,omitempty"`
	Status          string        `json:"status,omitempty"`
	Progress        *JSONProgress `json:"progressDetail,omitempty"`
	ProgressMessage string        `json:"progress,omitempty"` // deprecated
	ID              string        `json:"id,omitempty"`
	From            string        `json:"from,omitempty"`
	Time            int64         `json:"time,omitempty"`
	TimeNano        int64         `json:"timeNano,omitempty"`
	Error           *JSONError    `json:"errorDetail,omitempty"`
	ErrorMessage    string        `json:"error,omitempty"` // deprecated
	// Aux contains out-of-band data, such as digests for push signing and image id after building.
	Aux *json.RawMessage `json:"aux,omitempty"`
}

func clearLine(out io.Writer) {
	eraseMode := aec.EraseModes.All
	cl := aec.EraseLine(eraseMode)
	fmt.Fprint(out, cl)
}

func cursorUp(out io.Writer, l uint) {
	fmt.Fprint(out, aec.Up(l))
}

func cursorDown(out io.Writer, l uint) {
	fmt.Fprint(out, aec.Down(l))
}

// Display displays the JSONMessage to `out`. If `isTerminal` is true, it will erase the
// entire current line when displaying the progressbar.
func (jm *JSONMessage) Display(out io.Writer) error {
	if jm.Error != nil {
		if jm.Error.Code == 401 {
			return fmt.Errorf("authentication is required")
		}
		return jm.Error
	}
	var endl string
	if jm.Stream == "" && jm.Progress != nil {
		clearLine(out)
		endl = "\r"
		fmt.Fprint(out, endl)
	} else if jm.Progress != nil && jm.Progress.String() != "" { // disable progressbar in non-terminal
		return nil
	}
	if jm.TimeNano != 0 {
		fmt.Fprintf(out, "%s ", time.Unix(0, jm.TimeNano).Format(RFC3339NanoFixed))
	} else if jm.Time != 0 {
		fmt.Fprintf(out, "%s ", time.Unix(jm.Time, 0).Format(RFC3339NanoFixed))
	}
	if jm.ID != "" {
		fmt.Fprintf(out, "%s: ", jm.ID)
	}
	if jm.From != "" {
		fmt.Fprintf(out, "(from %s) ", jm.From)
	}
	if jm.Progress != nil {
		fmt.Fprintf(out, "%s %s%s", jm.Status, jm.Progress.String(), endl)
	} else if jm.ProgressMessage != "" { // deprecated
		fmt.Fprintf(out, "%s %s%s", jm.Status, jm.ProgressMessage, endl)
	} else if jm.Stream != "" {
		fmt.Fprintf(out, "%s%s", jm.Stream, endl)
	} else {
		fmt.Fprintf(out, "%s%s\r\n", jm.Status, endl)
	}
	return nil
}

// DisplayJSONMessagesStream displays a json message stream from `in` to `out`, `isTerminal`
// describes if `out` is a terminal. If this is the case, it will print `\n` at the end of
// each line and move the cursor while displaying.
func DisplayJSONMessagesStream(in io.Reader, out io.Writer, term *terminal.Terminal) error {
	var (
		dec = json.NewDecoder(in)
		ids = make(map[string]uint)
	)

	for {
		var diff uint
		var jm JSONMessage
		if err := dec.Decode(&jm); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if jm.Progress != nil {
			jm.Progress.Terminal = term
		}
		if jm.ID != "" && (jm.Progress != nil || jm.ProgressMessage != "") {
			line, ok := ids[jm.ID]
			if !ok {
				// NOTE: This approach of using len(id) to
				// figure out the number of lines of history
				// only works as long as we clear the history
				// when we output something that's not
				// accounted for in the map, such as a line
				// with no ID.
				line = uint(len(ids))
				ids[jm.ID] = line
				fmt.Fprintf(out, "\r\n")
			}
			diff = uint(len(ids)) - line
			cursorUp(out, diff)
		} else {
			// When outputting something that isn't progress
			// output, clear the history of previous lines. We
			// don't want progress entries from some previous
			// operation to be updated (for example, pull -a
			// with multiple tags).
			ids = make(map[string]uint)
		}
		err := jm.Display(out)
		if jm.ID != "" {
			cursorDown(out, diff)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
