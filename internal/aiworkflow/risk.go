package aiworkflow

import (
	"regexp"
	"strings"
)

var riskPriority = map[RiskLevel]int{
	RiskLevelLow:    0,
	RiskLevelMedium: 1,
	RiskLevelHigh:   2,
}

func DefaultRiskRules() []RiskRule {
	return []RiskRule{
		{Allow: true, Level: RiskLevelLow, Reason: "rm -rf in temp", Regex: `(?i)\brm\s+-rf\s+/(tmp|var/tmp)\b`},
		{Level: RiskLevelHigh, Reason: "rm -rf on root", Regex: `(?i)\brm\s+-rf\s+/`},
		{Level: RiskLevelHigh, Reason: "mkfs detected", Regex: `(?i)\bmkfs\b`},
		{Level: RiskLevelHigh, Reason: "shutdown/reboot", Regex: `(?i)\b(shutdown|reboot|poweroff|init\s+0)\b`},
		{Level: RiskLevelHigh, Reason: "wipe/format disk", Regex: `(?i)\b(wipefs|dd\s+if=.*of=/dev)\b`},
		{Level: RiskLevelMedium, Reason: "iptables flush", Regex: `(?i)\biptables\s+-F\b`},
		{Level: RiskLevelMedium, Reason: "user deletion", Regex: `(?i)\buserdel\b`},
		{Level: RiskLevelMedium, Reason: "chmod 777", Regex: `(?i)\bchmod\s+777\b`},
	}
}

func EvaluateRisk(text string, rules []RiskRule) (RiskLevel, []string) {
	allowRules := []RiskRule{}
	denyRules := []RiskRule{}
	for _, rule := range rules {
		if rule.Allow {
			allowRules = append(allowRules, rule)
		} else {
			denyRules = append(denyRules, rule)
		}
	}

	filtered := text
	if len(allowRules) > 0 {
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			cleaned := line
			for _, rule := range allowRules {
				re := regexp.MustCompile(rule.Regex)
				cleaned = re.ReplaceAllString(cleaned, "")
			}
			lines[i] = cleaned
		}
		filtered = strings.Join(lines, "\n")
	}

	level := RiskLevelLow
	notes := []string{}
	for _, rule := range denyRules {
		re := regexp.MustCompile(rule.Regex)
		if re.MatchString(filtered) {
			notes = append(notes, rule.Reason)
			level = maxRisk(level, rule.Level)
		}
	}
	return level, notes
}

func maxRisk(a, b RiskLevel) RiskLevel {
	if riskPriority[b] > riskPriority[a] {
		return b
	}
	return a
}
