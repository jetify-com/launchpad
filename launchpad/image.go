package launchpad

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type LocalImage string

func NewLocalImage(n string) *LocalImage {
	return newLocalImageWithTag(n, "")
}

func newLocalImageWithTag(n, t string) *LocalImage {
	image := n
	if n != "" && t != "" {
		image = n + ":" + t
	}
	li := LocalImage(image)
	return &li
}

func (l *LocalImage) Name() string {
	if l == nil {
		return ""
	}
	return strings.Split(l.String(), ":")[0]
}

func (l *LocalImage) Tag() string {
	if l == nil {
		return ""
	}
	if parts := strings.Split(l.String(), ":"); len(parts) > 1 {
		return parts[1]
	}
	return ""
}

func (l *LocalImage) String() string {
	if l == nil {
		return ""
	}
	return string(*l)
}

// generateDateImageTag returns:
// date-time as a series of 15 numbers that are unique each time a new image
// is built since it's the current date-time in UTC.
func generateDateImageTag(prefix string) string {
	// UTC is chosen because most timezones have daylight savings so some
	// Date-time combinations repeat and may cause duplicated tags.
	currentTime := time.Now().UTC()
	// This fomat has to have the "-", ":" and "." characters so that time
	// package is able to parse it. After formatting the date-time we remove them.
	formattedTime := fmt.Sprint(currentTime.Format("2006-01-02 15:04:05.0"))
	formattedTime = regexp.MustCompile(`[. \-:]`).ReplaceAllString(formattedTime, "")
	return prefix + formattedTime
}
