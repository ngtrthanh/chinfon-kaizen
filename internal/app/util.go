package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func contextWithTimeout(r *http.Request, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), d)
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func parseVND(s string) (int64, bool, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false, nil
	}
	replacer := strings.NewReplacer(",", "", ".", "", " ", "", "₫", "", "đ", "", "VNĐ", "", "VND", "")
	s = replacer.Replace(strings.ToUpper(s))
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < 0 {
		return 0, false, fmt.Errorf("giá trị tiền không hợp lệ")
	}
	return v, true, nil
}

func parsePercent(s string) (float64, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 || v > 100 {
		return 0, fmt.Errorf("tỷ lệ phải nằm trong 0–100")
	}
	return v, nil
}

func cleanCode(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func safeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "\x00", "")
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "\n", "")
	if name == "." || name == "" {
		return "file"
	}
	if len(name) > 180 {
		ext := filepath.Ext(name)
		name = name[:180-len(ext)] + ext
	}
	return name
}

func addBusinessDays(start time.Time, days int) time.Time {
	d := start
	added := 0
	for added < days {
		d = d.AddDate(0, 0, 1)
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			added++
		}
	}
	return d
}

func dueForTrack(track string, now time.Time) time.Time {
	switch track {
	case TrackFast48:
		return now.Add(48 * time.Hour)
	case TrackConfidential:
		return now.Add(24 * time.Hour)
	default:
		return addBusinessDays(now, 5)
	}
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

func parseAppTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func formatDate(s string) string {
	t, ok := parseAppTime(s)
	if !ok {
		return "—"
	}
	return t.Local().Format("02/01/2006")
}

func formatDateTime(s string) string {
	t, ok := parseAppTime(s)
	if !ok {
		return "—"
	}
	return t.Local().Format("02/01/2006 15:04")
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func requestIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func shortHash(s string) string {
	if len(s) > 16 {
		return s[:16]
	}
	return s
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func last5Digits(s string) string {
	d := digitsOnly(s)
	if len(d) < 5 {
		return ""
	}
	return d[len(d)-5:]
}

func firstRune(s string) string {
	for _, r := range strings.TrimSpace(s) {
		return strings.ToUpper(string(r))
	}
	return "?"
}
