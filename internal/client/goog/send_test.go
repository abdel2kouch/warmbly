package goog

import (
	"mime"
	"strings"
	"testing"
)

func TestBuildMixedMIMEEncodesUnicodeSubject(t *testing.T) {
	const subject = "Open When You’re Alone 😘"
	raw, err := buildMixedMIME([]header{{"Subject", mime.QEncoding.Encode("UTF-8", subject)}}, "hello", "", nil)
	if err != nil {
		t.Fatalf("build MIME: %v", err)
	}
	headers := string(raw[:strings.Index(string(raw), "\r\n\r\n")])
	if strings.Contains(headers, subject) {
		t.Fatalf("subject was written as raw UTF-8 header: %q", headers)
	}
	line := strings.TrimPrefix(strings.Split(headers, "\r\n")[0], "Subject: ")
	decoded, err := new(mime.WordDecoder).DecodeHeader(line)
	if err != nil {
		t.Fatalf("decode RFC 2047 subject: %v", err)
	}
	if decoded != subject {
		t.Fatalf("decoded subject = %q, want %q", decoded, subject)
	}
}
