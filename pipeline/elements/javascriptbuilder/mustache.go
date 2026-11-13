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
	"fmt"
	"strings"
)

// This file implements the part of Mustache that javascript-templates'
// JavaScriptResource.mustache uses, as the Mustache specification
// (https://github.com/mustache/spec) defines it. That is variables ({{x}},
// {{&x}} and {{{x}}}), sections ({{#x}}...{{/x}}), inverted sections
// ({{^x}}...{{/x}}) and comments ({{! x}}), with a line that holds only one
// of the last three tags and spaces or tabs removed, which the specification
// calls a standalone line. A closing tag closes the innermost open section,
// and its name is not checked. The template's sections are all booleans and
// it iterates no lists, so a full Mustache engine is unnecessary.

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
	// Values are never HTML-escaped: the template produces JavaScript, not HTML,
	// so all variable forms ({{x}}, {{&x}}, {{{x}}}) emit raw text.
	b.WriteString(stringify(val))
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

// parseMustache parses a template string into a renderable tree. A section
// that is not closed is left out, and a tag that is not closed is written as
// text.
func parseMustache(src string) *mustacheTemplate {
	root := &frame{}
	stack := []*frame{root}
	top := func() *frame { return stack[len(stack)-1] }
	add := func(n mustacheNode) { f := top(); f.children = append(f.children, n) }

	// text is where the text not yet added starts.
	text := 0
	for {
		open := strings.Index(src[text:], "{{")
		if open < 0 {
			break
		}
		open += text
		var tag string
		end := -1
		if strings.HasPrefix(src[open+2:], "{") {
			// Triple-brace unescaped variable: {{{ name }}}
			if close := strings.Index(src[open+3:], "}}}"); close >= 0 {
				tag = "&" + src[open+3:open+3+close]
				end = open + 3 + close + 3
			}
		} else if close := strings.Index(src[open+2:], "}}"); close >= 0 {
			tag = strings.TrimSpace(src[open+2 : open+2+close])
			end = open + 2 + close + 2
		}
		if end < 0 {
			break
		}

		// A standalone tag takes its line with it, from the line's start
		// to its line ending.
		before, after := open, end
		if strings.HasPrefix(tag, "#") || strings.HasPrefix(tag, "^") ||
			strings.HasPrefix(tag, "/") || strings.HasPrefix(tag, "!") {
			if start, ok := standaloneStart(src, text, open); ok {
				if next, ok := standaloneEnd(src, end); ok {
					before, after = start, next
				}
			}
		}
		if before > text {
			add(textNode(src[text:before]))
		}
		text = after

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
	if text < len(src) {
		add(textNode(src[text:]))
	}
	return &mustacheTemplate{root: root.children}
}

// standaloneStart returns where the line holding a tag that starts at open
// begins, if only spaces and tabs come between the two. The text added so far
// ends at from, so anything on the line before from is another tag.
func standaloneStart(src string, from, open int) (int, bool) {
	i := open
	for i > from && (src[i-1] == ' ' || src[i-1] == '\t') {
		i--
	}
	if i == 0 || src[i-1] == '\n' {
		return i, true
	}
	return 0, false
}

// standaloneEnd returns where the line after a tag that ends at end begins,
// past its LF or CRLF, if only spaces and tabs come before the line ending or
// the end of the template.
func standaloneEnd(src string, end int) (int, bool) {
	i := end
	for i < len(src) && (src[i] == ' ' || src[i] == '\t') {
		i++
	}
	switch {
	case i == len(src):
		return i, true
	case src[i] == '\n':
		return i + 1, true
	case strings.HasPrefix(src[i:], "\r\n"):
		return i + 2, true
	}
	return 0, false
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
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		// Covers numeric context values such as the sequence number, which must
		// render as their literal value (e.g. "var sequence = 1;").
		return fmt.Sprint(t)
	}
}
