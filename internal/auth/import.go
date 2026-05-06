package auth

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	curlHeaderPattern = regexp.MustCompile(`(?i)-H\s+(?:'([^']*)'|"([^"]*)")`)
	curlCookiePattern = regexp.MustCompile(`(?i)(?:--cookie|-b)\s+(?:'([^']*)'|"([^"]*)")`)
	projectPattern    = regexp.MustCompile(`(?i)["']?(project_id|projectId)["']?\s*[:=]\s*["']([^"',\s]+)`)
	cidPattern        = regexp.MustCompile(`(?i)["']?(cid|webid|webId)["']?\s*[:=]\s*["']([^"',\s]+)`)
)

// ParseImport parses JSON credentials, raw headers, or copied cURL text.
func ParseImport(data []byte) (Session, error) {
	if session, err := ParseJSON(data); err == nil {
		return session, nil
	}
	if session, err := ParseCurl(data); err == nil {
		return session, nil
	}
	return ParseHeaders(data)
}

// ParseJSON parses flat v2 credentials or nested reverse-tool credentials.
func ParseJSON(data []byte) (Session, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return Session{}, err
	}
	session := sessionFromMap(raw)
	if session.Cookie == "" && session.Token == "" {
		return Session{}, fmt.Errorf("auth: JSON did not contain cookie or token")
	}
	return session, nil
}

// ParseCurl parses headers and project hints from a copied cURL command.
func ParseCurl(data []byte) (Session, error) {
	text := string(data)
	matches := curlHeaderPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 && !curlCookiePattern.MatchString(text) {
		return Session{}, fmt.Errorf("auth: cURL did not contain headers")
	}
	var lines []string
	for _, match := range matches {
		if value := firstRegexGroup(match); value != "" {
			lines = append(lines, value)
		}
	}
	if match := curlCookiePattern.FindStringSubmatch(text); len(match) > 0 {
		if value := firstRegexGroup(match); value != "" {
			lines = append(lines, "cookie: "+value)
		}
	}
	session := sessionFromHeaderLines(lines)
	applyTextHints(text, &session)
	if session.Cookie == "" && session.Token == "" {
		return Session{}, fmt.Errorf("auth: cURL did not contain cookie or token")
	}
	return session, nil
}

func firstRegexGroup(match []string) string {
	for _, value := range match[1:] {
		if value != "" {
			return value
		}
	}
	return ""
}

// ParseHeaders parses newline-delimited HTTP headers.
func ParseHeaders(data []byte) (Session, error) {
	text := string(data)
	lines := strings.Split(text, "\n")
	session := sessionFromHeaderLines(lines)
	applyTextHints(text, &session)
	if session.Cookie == "" && session.Token == "" {
		return Session{}, fmt.Errorf("auth: headers did not contain cookie or token")
	}
	return session, nil
}

// MergeSession overlays non-empty override fields onto base.
func MergeSession(base, override Session) Session {
	if override.Cookie != "" {
		base.Cookie = override.Cookie
	}
	if override.Token != "" {
		base.Token = override.Token
	}
	if override.CSRF != "" {
		base.CSRF = override.CSRF
	}
	if override.ProjectID != "" {
		base.ProjectID = override.ProjectID
	}
	if override.CID != "" {
		base.CID = override.CID
	}
	if override.Source != "" {
		base.Source = override.Source
	}
	return base
}

func sessionFromHeaderLines(lines []string) Session {
	raw := map[string]any{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "-H ")
		line = strings.Trim(line, `'"`)
		if line == "" || !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key != "" && value != "" {
			raw[key] = value
		}
	}
	return sessionFromMap(map[string]any{"headers": raw})
}

func applyTextHints(text string, session *Session) {
	if session.ProjectID == "" {
		if match := projectPattern.FindStringSubmatch(text); len(match) >= 3 {
			session.ProjectID = match[2]
		}
	}
	if session.CID == "" {
		if match := cidPattern.FindStringSubmatch(text); len(match) >= 3 {
			session.CID = match[2]
		}
	}
}
