package reader

import (
	"strings"
)

func spaceSplit(s string) (parts []string) {
	for part := range strings.SplitSeq(strings.TrimSpace(s), " ") {
		if len(part) > 0 {
			parts = append(parts, part)
		}
	}
	return
}

func quoteSplit(s string) (parts []string, err error) {
	all := strings.Split(strings.TrimSpace(s), `"`)
	if len(all)%2 == 0 {
		return nil, ErrUnbalancedQuotes
	}
	for _, part := range all {
		if len(strings.TrimSpace(part)) > 0 {
			parts = append(parts, part)
		}
	}
	return parts, nil
}
