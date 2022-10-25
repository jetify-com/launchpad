package jetconfig

import (
	"strconv"
	"strings"
	"time"
)

var stamps = map[string]string{
	"$JETPACK_TIMESTAMP": strconv.Itoa(int(time.Now().Unix())),
}

func InterpolateStamps(s string) string {
	for k, v := range stamps {
		s = strings.ReplaceAll(s, k, v)
	}
	return s
}
