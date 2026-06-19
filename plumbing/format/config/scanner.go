package config

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

type configCallback func(section, subsection, key, value string, blank bool) error

type configScanner struct {
	src []rune
	pos int
}

func readConfigWithCallback(r io.Reader, cb configCallback) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if !utf8.Valid(b) {
		return errors.New("config: illegal UTF-8 encoding")
	}
	if bytes.IndexByte(b, 0) >= 0 {
		return errors.New("config: illegal character NUL")
	}

	s := &configScanner{src: []rune(string(b))}
	return s.scan(cb)
}

func (s *configScanner) scan(cb configCallback) error {
	var section, subsection string
	for {
		s.skipWhitespace()
		switch ch := s.peek(); {
		case ch < 0:
			return nil
		case ch == '\n':
			s.next()
		case ch == ';' || ch == '#':
			s.skipComment()
		case ch == '[':
			name, sub, err := s.scanSectionHeader()
			if err != nil {
				return err
			}
			section, subsection = name, sub
			if err := cb(section, subsection, "", "", true); err != nil {
				return err
			}
		case isConfigIdentStart(ch):
			if section == "" {
				return errors.New("config: expected section header")
			}
			key := s.scanIdentifier()
			s.skipWhitespace()
			blank := false
			value := ""
			switch ch := s.peek(); {
			case ch < 0 || ch == '\n' || ch == ';' || ch == '#':
				blank = true
				s.consumeLineEnd()
			case ch == '=':
				s.next()
				v, err := s.scanValue()
				if err != nil {
					return err
				}
				value = v
			default:
				return errors.New("config: expected '='")
			}
			if err := cb(section, subsection, key, value, blank); err != nil {
				return err
			}
		default:
			if section == "" {
				return errors.New("config: expected section header")
			}
			return errors.New("config: expected section header or variable declaration")
		}
	}
}

func (s *configScanner) scanSectionHeader() (string, string, error) {
	s.next()
	s.skipWhitespace()
	name := s.scanIdentifier()
	if name == "" {
		return "", "", errors.New("config: expected section name")
	}

	s.skipWhitespace()
	subsection := ""
	if s.peek() == '"' {
		sub, err := s.scanSubsection()
		if err != nil {
			return "", "", err
		}
		subsection = sub
		s.skipWhitespace()
	}
	if s.peek() != ']' {
		return "", "", errors.New("config: expected right bracket")
	}

	s.next()
	s.skipWhitespace()
	if ch := s.peek(); ch >= 0 && ch != '\n' && ch != ';' && ch != '#' {
		return "", "", errors.New("config: expected EOL, EOF, or comment")
	}
	s.consumeLineEnd()
	return name, subsection, nil
}

func (s *configScanner) scanSubsection() (string, error) {
	s.next()
	var b strings.Builder
	for {
		ch := s.next()
		switch ch {
		case -1, '\n':
			return "", errors.New("config: string not terminated")
		case '"':
			return b.String(), nil
		case '\\':
			esc := s.next()
			switch esc {
			case '\\', '"':
				b.WriteRune(esc)
			default:
				return "", errors.New("config: unknown escape sequence")
			}
		default:
			b.WriteRune(ch)
		}
	}
}

func (s *configScanner) scanValue() (string, error) {
	s.skipWhitespace()

	var b strings.Builder
	var pending strings.Builder
	inQuote := false
	for {
		ch := s.next()
		switch {
		case ch < 0:
			if inQuote {
				return "", errors.New("config: string not terminated")
			}
			return b.String(), nil
		case inQuote && ch == '\n':
			return "", errors.New("config: string not terminated")
		case !inQuote && (ch == '\n' || ch == ';' || ch == '#'):
			if ch == ';' || ch == '#' {
				s.skipComment()
			}
			return b.String(), nil
		case ch == '"':
			b.WriteString(pending.String())
			pending.Reset()
			inQuote = !inQuote
		case ch == '\\':
			if err := s.scanValueEscape(&b, &pending, inQuote); err != nil {
				return "", err
			}
		case !inQuote && ch == '\r':
		case !inQuote && isConfigWhitespace(ch):
			pending.WriteRune(ch)
		default:
			b.WriteString(pending.String())
			pending.Reset()
			b.WriteRune(ch)
		}
	}
}

func (s *configScanner) scanValueEscape(b, pending *strings.Builder, inQuote bool) error {
	esc := s.next()
	if !inQuote && esc == '\r' {
		esc = s.next()
	}
	if !inQuote && esc == '\n' {
		return nil
	}

	var ch rune
	switch esc {
	case '\\', '"':
		ch = esc
	case 'n':
		ch = '\n'
	case 't':
		ch = '\t'
	case 'b':
		ch = '\b'
	case '\n':
		ch = '\n'
	default:
		return errors.New("config: unknown escape sequence")
	}

	b.WriteString(pending.String())
	pending.Reset()
	b.WriteRune(ch)
	return nil
}

func (s *configScanner) scanIdentifier() string {
	if !isConfigIdentStart(s.peek()) {
		return ""
	}

	var b strings.Builder
	for isConfigIdent(s.peek()) {
		b.WriteRune(s.next())
	}
	return b.String()
}

func (s *configScanner) skipWhitespace() {
	for isConfigWhitespace(s.peek()) {
		s.next()
	}
}

func (s *configScanner) skipComment() {
	for {
		ch := s.peek()
		if ch < 0 || ch == '\n' {
			return
		}
		s.next()
	}
}

func (s *configScanner) consumeLineEnd() {
	if ch := s.peek(); ch == ';' || ch == '#' {
		s.skipComment()
	}
	if s.peek() == '\n' {
		s.next()
	}
}

func (s *configScanner) peek() rune {
	if s.pos >= len(s.src) {
		return -1
	}
	return s.src[s.pos]
}

func (s *configScanner) next() rune {
	ch := s.peek()
	if ch >= 0 {
		s.pos++
	}
	return ch
}

func isConfigIdentStart(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch >= 0x80 && unicode.IsLetter(ch)
}

func isConfigIdent(ch rune) bool {
	return isConfigIdentStart(ch) || '0' <= ch && ch <= '9' || ch >= 0x80 && unicode.IsDigit(ch) || ch == '-'
}

func isConfigWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\r'
}
