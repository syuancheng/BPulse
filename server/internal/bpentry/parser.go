package bpentry

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Candidate struct {
	Systolic  FieldCandidate `json:"systolic"`
	Diastolic FieldCandidate `json:"diastolic"`
	Pulse     FieldCandidate `json:"pulse"`
}

type FieldCandidate struct {
	Value      *int    `json:"value"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

type InterpretResult struct {
	Candidate         Candidate `json:"candidate"`
	NeedsConfirmation bool      `json:"needsConfirmation"`
}

var labelPatterns = map[string]*regexp.Regexp{
	"systolic":  regexp.MustCompile(`(?:高压|收缩压|上压|SYS|sys)[：:\s]*(?:是|为|约|大概)?[：:\s]*([一二三四五六七八九十百零〇两\d]{2,5})`),
	"diastolic": regexp.MustCompile(`(?:低压|舒张压|下压|DIA|dia)[：:\s]*(?:是|为|约|大概)?[：:\s]*([一二三四五六七八九十百零〇两\d]{2,5})`),
	"pulse":     regexp.MustCompile(`(?:心率|脉搏|PUL|pul|搏动|bpm)[：:\s]*(?:是|为|约|大概)?[：:\s]*([一二三四五六七八九十百零〇两\d]{2,5})`),
}

func InterpretText(text string) (InterpretResult, error) {
	normalized := strings.TrimSpace(text)
	if normalized == "" || len([]rune(normalized)) > 500 {
		return InterpretResult{}, fmt.Errorf("recognized text is invalid")
	}
	return InterpretResult{
		Candidate: Candidate{
			Systolic:  parseField(normalized, "systolic"),
			Diastolic: parseField(normalized, "diastolic"),
			Pulse:     parseField(normalized, "pulse"),
		},
		NeedsConfirmation: true,
	}, nil
}

func parseField(text, field string) FieldCandidate {
	pattern := labelPatterns[field]
	matches := pattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return FieldCandidate{Reason: "missing_label"}
	}
	values := map[int]struct{}{}
	for _, match := range matches {
		value, ok := parseNumber(match[1])
		if !ok {
			continue
		}
		values[value] = struct{}{}
	}
	if len(values) == 0 {
		return FieldCandidate{Reason: "unreadable"}
	}
	if len(values) > 1 {
		return FieldCandidate{Reason: "conflict"}
	}
	for value := range values {
		if !inFieldBounds(field, value) {
			return FieldCandidate{Reason: "out_of_bounds"}
		}
		v := value
		return FieldCandidate{Value: &v, Confidence: 0.92, Reason: "matched_label"}
	}
	return FieldCandidate{Reason: "unreadable"}
}

func inFieldBounds(field string, value int) bool {
	switch field {
	case "systolic":
		return value >= 40 && value <= 260
	case "diastolic":
		return value >= 30 && value <= 180
	case "pulse":
		return value >= 30 && value <= 220
	default:
		return false
	}
}

func parseNumber(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	if value, err := strconv.Atoi(raw); err == nil {
		return value, true
	}
	if strings.Contains(raw, "百") {
		parts := strings.SplitN(raw, "百", 2)
		hundredsRaw := parts[0]
		if hundredsRaw == "" {
			hundredsRaw = "一"
		}
		hundreds, ok := parseSmallChineseNumber(hundredsRaw)
		if !ok {
			return 0, false
		}
		restRaw := ""
		if len(parts) == 2 {
			restRaw = parts[1]
		}
		if restRaw == "" {
			return hundreds * 100, true
		}
		if len([]rune(restRaw)) == 1 && regexp.MustCompile(`^[一二两三四五六七八九]$`).MatchString(restRaw) {
			rest, ok := parseSmallChineseNumber(restRaw)
			if !ok {
				return 0, false
			}
			return hundreds*100 + rest*10, true
		}
		rest, ok := parseSmallChineseNumber(restRaw)
		if !ok {
			return 0, false
		}
		return hundreds*100 + rest, true
	}
	return parseSmallChineseNumber(raw)
}

func parseSmallChineseNumber(raw string) (int, bool) {
	digits := map[rune]int{'零': 0, '〇': 0, '一': 1, '二': 2, '两': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	total := 0
	current := 0
	for _, r := range raw {
		switch r {
		case '百':
			if current == 0 {
				current = 1
			}
			total += current * 100
			current = 0
		case '十':
			if current == 0 {
				current = 1
			}
			total += current * 10
			current = 0
		default:
			value, ok := digits[r]
			if !ok {
				return 0, false
			}
			current = value
		}
	}
	total += current
	if total == 0 {
		return 0, false
	}
	return total, true
}
