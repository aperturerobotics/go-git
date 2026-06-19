package config

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type DecoderSuite struct {
	suite.Suite
}

func TestDecoderSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(DecoderSuite))
}

func (s *DecoderSuite) TestDecode() {
	for idx, fixture := range fixtures {
		r := bytes.NewReader([]byte(fixture.Raw))
		d := NewDecoder(r)
		cfg := &Config{}
		err := d.Decode(cfg)
		s.NoError(err, fmt.Sprintf("decoder error for fixture: %d", idx))
		buf := bytes.NewBuffer(nil)
		e := NewEncoder(buf)
		_ = e.Encode(cfg)
		s.Equal(fixture.Config, cfg, fmt.Sprintf("bad result for fixture: %d, %s", idx, buf.String()))
	}
}

func (s *DecoderSuite) TestDecodeFailsWithIdentBeforeSection() {
	t := `
	key=value
	[section]
	key=value
	`
	decodeFails(s, t)
}

func (s *DecoderSuite) TestDecodeFailsWithEmptySectionName() {
	t := `
	[]
	key=value
	`
	decodeFails(s, t)
}

func (s *DecoderSuite) TestDecodeSucceedsWithEmptySubsectionName() {
	t := `
	[remote ""]
	key=value
	`
	decodeSucceeds(s, t)
}

func (s *DecoderSuite) TestDecodeFailsWithBadSubsectionName() {
	t := `
	[remote origin"]
	key=value
	`
	decodeFails(s, t)
	t = `
	[remote "origin]
	key=value
	`
	decodeFails(s, t)
}

func (s *DecoderSuite) TestDecodeFailsWithTrailingGarbage() {
	t := `
	[remote]garbage
	key=value
	`
	decodeFails(s, t)
	t = `
	[remote "origin"]garbage
	key=value
	`
	decodeFails(s, t)
}

func (s *DecoderSuite) TestDecodeFailsWithGarbage() {
	decodeFails(s, "---")
	decodeFails(s, "????")
	decodeFails(s, "[sect\nkey=value")
	decodeFails(s, "sect]\nkey=value")
	decodeFails(s, `[section]key="value`)
	decodeFails(s, `[section]key=value"`)
}

func (s *DecoderSuite) TestDecodeFailsWithInvalidBytes() {
	decodeBytesFail(s, []byte("[section]\nkey=va\x00lue\n"))
	decodeBytesFail(s, []byte("[section]\nkey=va\xfflue\n"))
}

func decodeFails(s *DecoderSuite, text string) {
	decodeBytesFail(s, []byte(text))
}

func decodeBytesFail(s *DecoderSuite, text []byte) {
	r := bytes.NewReader(text)
	d := NewDecoder(r)
	cfg := &Config{}
	err := d.Decode(cfg)
	s.NotNil(err)
}

func decodeSucceeds(s *DecoderSuite, text string) {
	r := bytes.NewReader([]byte(text))
	d := NewDecoder(r)
	cfg := &Config{}
	err := d.Decode(cfg)
	s.NoError(err)

	s.True(cfg.HasSection("remote"))
	remote := cfg.Section("remote")
	s.True(remote.HasOption("key"))
	s.Equal("value", remote.Option("key"))
}

func (s *DecoderSuite) TestDecodeScannerSyntax() {
	text := `[section]
	valueless
	continued = first\
second
	backspace = a\bz
	quoted-whitespace = "  kept  "
	inline-comment = value ; ignored
	quoted-comment = "value ; kept # kept"
[section "sub\"\\section"]
	key = value
`

	d := NewDecoder(bytes.NewReader([]byte(text)))
	cfg := &Config{}
	s.NoError(d.Decode(cfg))

	section := cfg.Section("section")
	s.True(section.HasOption("valueless"))
	s.Equal("", section.Option("valueless"))
	s.Equal("firstsecond", section.Option("continued"))
	s.Equal("a\bz", section.Option("backspace"))
	s.Equal("  kept  ", section.Option("quoted-whitespace"))
	s.Equal("value", section.Option("inline-comment"))
	s.Equal("value ; kept # kept", section.Option("quoted-comment"))

	subsection := section.Subsection(`sub"\section`)
	s.Equal("value", subsection.Option("key"))
}

func FuzzDecoder(f *testing.F) {
	f.Fuzz(func(_ *testing.T, input []byte) {
		d := NewDecoder(bytes.NewReader(input))
		cfg := &Config{}
		d.Decode(cfg)
	})
}
