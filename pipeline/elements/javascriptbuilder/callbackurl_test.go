/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2026 51 Degrees Mobile Experts Limited, Davidson House,
 * Forbury Square, Reading, Berkshire, United Kingdom RG1 3EU.
 *
 * This Original Work is licensed under the European Union Public Licence
 * (EUPL) v.1.2 and is subject to its terms as set out below.
 *
 * If a copy of the EUPL was not distributed with this file, You can obtain
 * one at https://opensource.org/licenses/EUPL-1.2.
 *
 * The 'Compatible Licences' set out in the Appendix to the EUPL (as may be
 * amended by the European Commission) shall be deemed incompatible for
 * the purposes of the Work and the provisions of the compatibility
 * clause in Article 5 of the EUPL shall not apply.
 *
 * If using the Work as, or as part of, a network application, by
 * including the attribution notice(s) required under Article 5 of the EUPL
 * in the end user terms of the application under an appropriate heading,
 * such notice(s) shall fulfill the requirements of that article.
 * ********************************************************************* */

package javascriptbuilder

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/jsonbuilder"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/sequence"
)

const testEndpoint = "/51dpipeline/json"

// scriptURL processes the evidence in a pipeline holding the element and
// returns the callback URL the script posts to, or "" when it has none.
func scriptURL(t *testing.T, e *Element, evidence map[string]string) string {
	t.Helper()
	p, err := core.NewPipelineBuilder().
		AddFlowElement(sequence.New(nil)).
		AddFlowElement(jsonbuilder.New(nil, nil)).
		AddFlowElement(e).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	for k, v := range evidence {
		fd.AddEvidence(k, v)
	}
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	js := data.(*Data).JavaScript()
	const call = "createCORSRequest('POST', '"
	i := strings.Index(js, call)
	if i < 0 {
		return ""
	}
	rest := js[i+len(call):]
	return rest[:strings.Index(rest, "')")]
}

// hostURL is the callback URL for a host from evidence, with the protocol
// https and the endpoint /51dpipeline/json.
func hostURL(t *testing.T, host string) string {
	t.Helper()
	return scriptURL(t, NewBuilder().SetEndpoint(testEndpoint).Build(),
		map[string]string{"header.protocol": "https", "header.host": host})
}

func TestCallbackURLAcceptedHosts(t *testing.T) {
	const tail = "/51dpipeline/json"
	label63, label64 := strings.Repeat("a", 63), strings.Repeat("a", 64)
	cases := []struct{ host, want string }{
		{"EXAMPLE.com", "https://example.com" + tail},
		{"example.com:8080", "https://example.com:8080" + tail},
		{"example.com:65535", "https://example.com:65535" + tail},
		{"example.com:00080", "https://example.com:00080" + tail},
		{"example.com:0065535", "https://example.com:0065535" + tail},
		{"a_b.com", "https://a_b.com" + tail},
		{"-ab.com", "https://-ab.com" + tail},
		{"ab-.com", "https://ab-.com" + tail},
		{"a-b.com", "https://a-b.com" + tail},
		{"localhost", "https://localhost" + tail},
		{"a.b.c.d.e.f", "https://a.b.c.d.e.f" + tail},
		{"xn--bcher-kva.de", "https://xn--bcher-kva.de" + tail},
		{label63 + ".com", "https://" + label63 + ".com" + tail},
		{label64 + ".com", "https://" + label64 + ".com" + tail},
		{"example.com.", "https://example.com." + tail},
		{"example.co1", "https://example.co1" + tail},
		{"123.example.com", "https://123.example.com" + tail},
		{"127.0.0.1", "https://127.0.0.1" + tail},
		{"127.0.0.1:80", "https://127.0.0.1:80" + tail},
		{"1.2.3.4:8080", "https://1.2.3.4:8080" + tail},
		{"0.0.0.0", "https://0.0.0.0" + tail},
		{"101.1.1.1", "https://101.1.1.1" + tail},
		{"255.255.255.255", "https://255.255.255.255" + tail},
		{"[::1]", "https://[::1]" + tail},
		{"[::1]:8080", "https://[::1]:8080" + tail},
		{"[FE80::1]", "https://[fe80::1]" + tail},
		{"[::ffff:1.2.3.4]", "https://[::ffff:1.2.3.4]" + tail},
		{"example.com/", "https://example.com" + tail},
		{"example.com/path", "https://example.com/path" + tail},
		{"example.com:8080/x", "https://example.com:8080/x" + tail},
		{"1.2.3.4/path", "https://1.2.3.4/path" + tail},
		{"EXAMPLE.com/Path", "https://example.com/Path" + tail},
		// Rule 8, after the authority.
		{"example.com/a b", "https://example.com/a%20b" + tail},
		{"example.com/a\tb", "https://example.com/a%09b" + tail},
		{"example.com/a\nb", "https://example.com/a%0Ab" + tail},
		{"example.com/a\x7fb", "https://example.com/a%7Fb" + tail},
		{"example.com/\u00fc", "https://example.com/%C3%BC" + tail},
		{"example.com/a\u2028b", "https://example.com/a%E2%80%A8b" + tail},
		{"example.com/a%zzb", "https://example.com/a%25zzb" + tail},
		{"example.com/a%41b", "https://example.com/a%41b" + tail},
		{"example.com/a%41", "https://example.com/a%41" + tail},
		{"example.com/a%", "https://example.com/a%25" + tail},
		{"example.com/a%4", "https://example.com/a%254" + tail},
		{"example.com/a'b", "https://example.com/a%27b" + tail},
		{"example.com/a\"b", "https://example.com/a%22b" + tail},
		{"example.com/a\\b", "https://example.com/a%5Cb" + tail},
		{"example.com/</script>", "https://example.com/%3C/script%3E" + tail},
		{"example.com/\"^`{|}", "https://example.com/%22%5E%60%7B%7C%7D" + tail},
		{"example.com/');alert(1);//", "https://example.com/%27);alert(1);/" + tail},
	}
	for _, c := range cases {
		t.Run(c.host, func(t *testing.T) {
			if got := hostURL(t, c.host); got != c.want {
				t.Errorf("host %q gave %q, want %q", c.host, got, c.want)
			}
		})
	}
}

func TestCallbackURLRefusedHosts(t *testing.T) {
	hosts := []string{
		// Rule 2, a query or fragment.
		"example.com?x", "example.com#x", "example.com?", "example.com#",
		"example.com:8080?x", "[::1]#x", "example.com/path?x",
		"example.com/path#x", "example.com?');alert(1);//",
		"example.com#');alert(1);//",
		// Rule 3, an @ anywhere.
		"user@example.com", "user:pw@example.com", "@example.com",
		":@example.com", "user@example.com:8080", "user@[::1]",
		"user@example.com/path", "u%41@example.com", "u x@example.com",
		"u\tx@example.com", "u\"x@example.com", "u'x@example.com",
		"u\\x@example.com", "u\nx@example.com", "u\x01x@example.com",
		"a@b@example.com", "example.com/a@b", "user:12/ss@example.com",
		"user:pa/ss@example.com", "user:pa?ss@example.com",
		// Rule 4, an IPv6 address in brackets and nothing else.
		"::1", "[::1", "[v1.x]", "[1.2.3.4]", "[::1]x", "[::1]80",
		"[::1]:80x", "[::1]]", "[fe80::1%25]", "[fe80::1%25'+alert(1)+']",
		"[fe80::1%eth0]", "[fe80::1%25eth%0]", "[fe80::1%25eth0]",
		"[fe80::1%25eth0]:8080", "[FE80::1%25ETH0]", "[fe80::1%251]",
		"[::1%25lo]", "[fe80::1%]", "[fe80::1%25eth0]/path",
		// Rule 4, a name of ASCII letters, digits, - and _ in labels.
		"exa mple.com", "example.com');alert(1);//", "a\"b.com", "a'b.com",
		"a<b.com", "a>b.com", "a..b.com", "%41.com", "a\\b.com", "a`b.com",
		"a{b}.com", "a|b.com", "a^b.com", "a(b).com", "a;b.com", "a,b.com",
		"a!b.com", "a*b.com", "a+b.com", "a=b.com", "a$b.com", "a&b.com",
		"a~b.com", "a\tb.com", "a\nb.com", " example.com", "example.com ",
		"example.com:8080');alert(1);//", "example.com..", ".", "/path",
		":8080", "b\u00fccher.de", "a\u00a0b.com", "a\u200bb.com",
		"a\u2028b.com", "\u65e5\u672c.jp", "\u00e9xample.com",
		"\u0430pple.com",
		// Rule 4, a name ending in a number must be an IPv4 address.
		"example.123", "example.0x1f", "example.0X", "1.2.3.999", "1.2.3",
		"127.1", "0x7f.0.0.1", "01.2.3.4", "00.1.2.3", "example.123.",
		"1.2.3.4.", "1.2.3.4.5", "1", "300.1.1.1", "0x7f.1",
		// Rule 5, a port of no more than 65535.
		"example.com:65536", "example.com:99999", "example.com:abc",
		"example.com:0065536", "example.com:123456",
		"example.com:4294967296", "example.com:99999999999999999999",
		// Rule 5, a port that is present but empty. A host value holding a
		// scheme is read as the host "https" with an empty port.
		"example.com:", "[::1]:", "https://example.com",
	}
	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			if got := hostURL(t, host); got != "" {
				t.Errorf("host %q gave %q, want no callback", host, got)
			}
		})
	}
}

// Rule 1 and rule 7, http or https ignoring ASCII case only, written in
// lower case.
func TestCallbackURLProtocols(t *testing.T) {
	const tail = "://example.com/51dpipeline/json"
	cases := []struct{ protocol, want string }{
		{"https", "https" + tail},
		{"http", "http" + tail},
		{"HtTp", "http" + tail},
		{"HTTPS", "https" + tail},
		{"ftp", ""}, {"h+t-p.s", ""}, {"ws", ""}, {"javascript", ""},
		{"file", ""}, {"data", ""}, {"blob", ""}, {"about", ""},
		{"httpx", ""}, {"https ", ""}, {" https", ""}, {"1http", ""},
		{"h t", ""}, {"x');alert(1);//", ""}, {"a://b", ""},
		{"http\u017f", ""}, {"HTTP\u017f", ""},
	}
	for _, c := range cases {
		t.Run(c.protocol, func(t *testing.T) {
			got := scriptURL(t, NewBuilder().SetEndpoint(testEndpoint).Build(),
				map[string]string{
					"header.protocol": c.protocol,
					"header.host":     "example.com",
				})
			if got != c.want {
				t.Errorf("protocol %q gave %q, want %q", c.protocol, got, c.want)
			}
		})
	}
}

// Step 6, one slash between the host and the endpoint, and rule 8 in the
// endpoint.
func TestCallbackURLConfigured(t *testing.T) {
	cases := []struct{ host, endpoint, want string }{
		{"example.com", "51dpipeline/json", "https://example.com/51dpipeline/json"},
		{"example.com", "/51dpipeline/json", "https://example.com/51dpipeline/json"},
		{"example.com/", "/json", "https://example.com/json"},
		{"example.com/", "json", "https://example.com/json"},
		{"example.com//", "/json", "https://example.com//json"},
		{"example.com", "/json%", "https://example.com/json%25"},
		{"example.com", "/json'x", "https://example.com/json%27x"},
		{"example.com", "/a b/\u00fc", "https://example.com/a%20b/%C3%BC"},
		{"example.com/\"^`{|}", "/j\"^`{|}",
			"https://example.com/%22%5E%60%7B%7C%7D/j%22%5E%60%7B%7C%7D"},
		{"user:12/ss@example.com", "/json", ""},
		{"https://example.com", "/json", ""},
	}
	for _, c := range cases {
		t.Run(c.host+" "+c.endpoint, func(t *testing.T) {
			e := NewBuilder().SetProtocol("https").SetHost(c.host).
				SetEndpoint(c.endpoint).Build()
			if got := scriptURL(t, e, nil); got != c.want {
				t.Errorf("host %q endpoint %q gave %q, want %q",
					c.host, c.endpoint, got, c.want)
			}
		})
	}
}

// Step 6, a protocol, host or endpoint that is empty or only white space
// gives no callback and is not logged.
func TestCallbackURLMissing(t *testing.T) {
	cases := []struct{ protocol, host, endpoint string }{
		{"https", "", testEndpoint},
		{"https", " ", testEndpoint},
		{"https", "\u0085", testEndpoint},
		{"https", "\u00a0\u3000", testEndpoint},
		{"https", "\u2028\t", testEndpoint},
		{" ", "example.com", testEndpoint},
		{"\u2003", "example.com", testEndpoint},
		{"https", "example.com", ""},
		{"https", "example.com", " "},
		{"https", "example.com", "\u205f"},
	}
	for _, c := range cases {
		t.Run(c.protocol+"|"+c.host+"|"+c.endpoint, func(t *testing.T) {
			h := &recordingHandler{}
			e := NewBuilder().SetLogger(slog.New(h)).
				SetEndpoint(c.endpoint).Build()
			got := scriptURL(t, e, map[string]string{
				"header.protocol": c.protocol,
				"header.host":     c.host,
			})
			if got != "" {
				t.Errorf("gave %q, want no callback", got)
			}
			if n := len(h.records()); n != 0 {
				t.Errorf("logged %d records, want none: %q", n, h.records())
			}
		})
	}
}

// Rule 1, a configured protocol that is not http or https is refused when
// the element is built.
func TestCallbackURLConfiguredProtocol(t *testing.T) {
	for _, protocol := range []string{
		"ftp", "javascript", " ", "\u00a0", "https ", "http\u017f",
	} {
		t.Run(protocol, func(t *testing.T) {
			var ce *core.ConfigurationError
			_, err := Factory(map[string]any{"Protocol": protocol}, nil)
			if !errors.As(err, &ce) {
				t.Errorf("Factory gave %v, want a configuration error", err)
			}
			p, err := core.NewPipelineBuilder().
				AddFlowElement(NewBuilder().SetProtocol(protocol).Build()).
				Build()
			if p != nil {
				p.Close()
			}
			if !errors.As(err, &ce) {
				t.Errorf("pipeline build gave %v, want a configuration error", err)
			}
		})
	}
	for _, c := range []struct{ protocol, want string }{
		{"", "https://example.com/51dpipeline/json"},
		{"HTTP", "http://example.com/51dpipeline/json"},
	} {
		t.Run(c.protocol, func(t *testing.T) {
			el, err := Factory(map[string]any{
				"Protocol": c.protocol,
				"Endpoint": testEndpoint,
			}, nil)
			if err != nil {
				t.Fatalf("Factory: %v", err)
			}
			got := scriptURL(t, el.(*Element), map[string]string{
				"header.host": "example.com",
			})
			if got != c.want {
				t.Errorf("protocol %q gave %q, want %q", c.protocol, got, c.want)
			}
		})
	}
}

// Rule 6, a refused URL is logged once by each element, with anything in
// the host value before its last @ shown as ***, and on one line.
func TestCallbackURLLogged(t *testing.T) {
	cases := []struct{ host, logged string }{
		{"user:pa/ss@example.com", "https://***@example.com/51dpipeline/json"},
		{"user:pa?ss@example.com", "https://***@example.com/51dpipeline/json"},
		{"exa mple.com/a@b", "https://***@b/51dpipeline/json"},
		{"u@exa mple.com/a@b/c", "https://***@b/c/51dpipeline/json"},
		{"a@b@example.com", "https://***@example.com/51dpipeline/json"},
		{"exa mple.com/a", "https://exa mple.com/a/51dpipeline/json"},
	}
	for _, c := range cases {
		t.Run(c.host, func(t *testing.T) {
			h := &recordingHandler{}
			e := NewBuilder().SetLogger(slog.New(h)).
				SetEndpoint(testEndpoint).Build()
			scriptURL(t, e, map[string]string{
				"header.protocol": "https", "header.host": c.host})
			records := h.records()
			if len(records) != 1 {
				t.Fatalf("logged %d records, want 1: %q", len(records), records)
			}
			if !strings.Contains(records[0], c.logged) {
				t.Errorf("logged %q, want it to hold %q", records[0], c.logged)
			}
			if at := strings.LastIndex(c.host, "@"); at > 0 &&
				strings.Contains(records[0], c.host[:at]) {
				t.Errorf("logged %q, which holds %q", records[0], c.host[:at])
			}
		})
	}

	t.Run("once per element", func(t *testing.T) {
		h := &recordingHandler{}
		logger := slog.New(h)
		first := NewBuilder().SetLogger(logger).SetEndpoint(testEndpoint).Build()
		second := NewBuilder().SetLogger(logger).SetEndpoint(testEndpoint).Build()
		for _, host := range []string{"a b", "example.123", "u@example.com"} {
			for _, e := range []*Element{first, second} {
				scriptURL(t, e, map[string]string{
					"header.protocol": "https", "header.host": host})
			}
		}
		if n := len(h.records()); n != 2 {
			t.Errorf("logged %d records, want 2: %q", n, h.records())
		}
	})

	t.Run("on one line", func(t *testing.T) {
		for _, host := range []string{
			"exa\nmple.com", "exa\rmple.com", "exa\u2028mple.com",
			"exa\u2029mple.com", "exa\u0085mple.com",
		} {
			h := &recordingHandler{}
			scriptURL(t, NewBuilder().SetLogger(slog.New(h)).
				SetEndpoint(testEndpoint).Build(),
				map[string]string{"header.protocol": "https", "header.host": host})
			records := h.records()
			if len(records) != 1 {
				t.Fatalf("host %q logged %d records, want 1", host, len(records))
			}
			if strings.ContainsAny(records[0], "\n\r\u2028\u2029\u0085") {
				t.Errorf("host %q logged %q, which breaks the line", host, records[0])
			}
		}
	})

	t.Run("not when valid", func(t *testing.T) {
		h := &recordingHandler{}
		scriptURL(t, NewBuilder().SetLogger(slog.New(h)).
			SetEndpoint(testEndpoint).Build(),
			map[string]string{"header.protocol": "https", "header.host": "example.com"})
		if n := len(h.records()); n != 0 {
			t.Errorf("logged %d records, want none: %q", n, h.records())
		}
	})
}

// recordingHandler keeps the text of each error record as the handler
// receives it, with no escaping of its own.
type recordingHandler struct {
	mu   sync.Mutex
	text []string
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	if r.Level < slog.LevelError {
		return nil
	}
	var b strings.Builder
	b.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		b.WriteString(" " + a.Key + "=" + a.Value.String())
		return true
	})
	h.mu.Lock()
	h.text = append(h.text, b.String())
	h.mu.Unlock()
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler      { return h }

func (h *recordingHandler) records() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.text...)
}
