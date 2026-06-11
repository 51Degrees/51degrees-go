package javascriptbuilder

import (
	"html"
	"strings"
)

// This file implements the small subset of Mustache used by the vendored
// JavaScript template (template.mustache): variable substitution ({{x}},
// {{&x}}, {{{x}}}), sections ({{#x}}...{{/x}}) and inverted sections
// ({{^x}}...{{/x}}). The template's section keys are all booleans and it does
// not iterate lists at the top level, so a full Mustache engine is unnecessary.
// See https://mustache.github.io/

type mustacheNode interface {
	render(ctx map[string]any, b *strings.Builder)
}

type textNode string

func (t textNode) render(_ map[string]any, b *strings.Builder) { b.WriteString(string(t)) }

type varNode struct {
	name     string
	unescape bool
}

func (v varNode) render(ctx map[string]any, b *strings.Builder) {
	val, ok := ctx[v.name]
	if !ok || val == nil {
		return
	}
	s := stringify(val)
	if v.unescape {
		b.WriteString(s)
	} else {
		b.WriteString(html.EscapeString(s))
	}
}

type sectionNode struct {
	name     string
	inverted bool
	children []mustacheNode
}

func (s sectionNode) render(ctx map[string]any, b *strings.Builder) {
	truthy := isTruthy(ctx[s.name])
	if truthy == s.inverted {
		return
	}
	for _, c := range s.children {
		c.render(ctx, b)
	}
}

// mustacheTemplate is a parsed template.
type mustacheTemplate struct {
	root []mustacheNode
}

func (t *mustacheTemplate) render(ctx map[string]any) string {
	var b strings.Builder
	for _, n := range t.root {
		n.render(ctx, &b)
	}
	return b.String()
}

// frame is used while parsing to track nested sections.
type frame struct {
	name     string
	inverted bool
	children []mustacheNode
}

// parseMustache parses a template string into a renderable tree. It returns an
// error string via panic-free best-effort: unbalanced sections render whatever
// was parsed.
func parseMustache(src string) *mustacheTemplate {
	root := &frame{}
	stack := []*frame{root}
	top := func() *frame { return stack[len(stack)-1] }
	add := func(n mustacheNode) { f := top(); f.children = append(f.children, n) }

	for len(src) > 0 {
		open := strings.Index(src, "{{")
		if open < 0 {
			add(textNode(src))
			break
		}
		if open > 0 {
			add(textNode(src[:open]))
		}
		src = src[open+2:]

		// Triple-brace unescaped variable: {{{ name }}}
		if strings.HasPrefix(src, "{") {
			src = src[1:]
			close := strings.Index(src, "}}}")
			if close < 0 {
				add(textNode("{{{" + src))
				break
			}
			name := strings.TrimSpace(src[:close])
			add(varNode{name: name, unescape: true})
			src = src[close+3:]
			continue
		}

		close := strings.Index(src, "}}")
		if close < 0 {
			add(textNode("{{" + src))
			break
		}
		tag := strings.TrimSpace(src[:close])
		src = src[close+2:]

		switch {
		case strings.HasPrefix(tag, "#"):
			stack = append(stack, &frame{name: strings.TrimSpace(tag[1:])})
		case strings.HasPrefix(tag, "^"):
			stack = append(stack, &frame{name: strings.TrimSpace(tag[1:]), inverted: true})
		case strings.HasPrefix(tag, "/"):
			if len(stack) > 1 {
				f := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				top().children = append(top().children, sectionNode{name: f.name, inverted: f.inverted, children: f.children})
			}
		case strings.HasPrefix(tag, "&"):
			add(varNode{name: strings.TrimSpace(tag[1:]), unescape: true})
		case strings.HasPrefix(tag, "!"):
			// comment: ignore
		default:
			add(varNode{name: tag})
		}
	}
	return &mustacheTemplate{root: root.children}
}

func isTruthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return t != ""
	default:
		return true
	}
}

func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}
