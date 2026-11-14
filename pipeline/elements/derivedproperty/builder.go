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

package derivedproperty

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// Builder builds a Derived Property Element from scripts given in three ways,
// which can be mixed: scripts shipped in this package, files on disk, and
// text in code. There is no way to fetch a script over the network.
type Builder struct {
	logger  *slog.Logger
	scripts []scriptText
	faults  []Fault
}

// scriptText is a script as given, before it is validated.
type scriptText struct {
	// name is the name the script must have, or empty for none.
	name   string
	text   string
	source string
}

// NewBuilder creates a Derived Property Element builder.
func NewBuilder() *Builder { return &Builder{} }

// SetLogger sets the logger.
func (b *Builder) SetLogger(l *slog.Logger) *Builder { b.logger = l; return b }

// AddBuiltInScript adds a script shipped in this package.
func (b *Builder) AddBuiltInScript(s BuiltInScript) *Builder {
	text, ok := s.text()
	if !ok {
		b.faults = append(b.faults, Fault{Script: string(s), Source: string(s), Message: fmt.Sprintf(
			"'%s' is not a built-in script. The built-in scripts are %s", s, builtInNames())})
		return b
	}
	b.scripts = append(b.scripts, scriptText{name: string(s), text: text, source: string(s)})
	return b
}

// AddScriptFile adds every file the path matches, where the path may hold
// wildcards as filepath.Glob reads them. A script's Name must equal its file
// name without the extension.
func (b *Builder) AddScriptFile(path string) *Builder {
	matches, err := filepath.Glob(path)
	if err != nil || len(matches) == 0 {
		b.faults = append(b.faults, Fault{Source: path, Message: fmt.Sprintf("no file matches '%s'", path)})
		return b
	}
	sort.Strings(matches)
	for _, match := range matches {
		name := strings.TrimSuffix(filepath.Base(match), filepath.Ext(match))
		data, err := os.ReadFile(match)
		if err != nil {
			b.faults = append(b.faults, Fault{Script: name, Source: match, Message: "the file cannot be read: " + err.Error()})
			continue
		}
		b.scripts = append(b.scripts, scriptText{name: name, text: string(data), source: match})
	}
	return b
}

// AddScript adds a script from text in code, read as YAML or JSON as the text
// shows. name is the name the script must have.
func (b *Builder) AddScript(name, content string) *Builder {
	b.scripts = append(b.scripts, scriptText{name: name, text: content, source: "code"})
	return b
}

// Build validates every script and builds the Element. Where any script has a
// fault, Build returns a ValidationError listing every fault in every script.
func (b *Builder) Build() (*Element, error) {
	if len(b.scripts) == 0 && len(b.faults) == 0 {
		return nil, errors.New("a Derived Property Element needs at least one script")
	}
	faults := append([]Fault{}, b.faults...)
	var models []*scriptModel
	for _, s := range b.scripts {
		model, f := validateText(s.text, s.name, s.source)
		faults = append(faults, f...)
		if model != nil {
			models = append(models, model)
		}
	}
	faults = append(faults, duplicates(models)...)
	if len(faults) > 0 {
		return nil, &ValidationError{Faults: faults}
	}

	logger := b.logger
	if logger == nil {
		logger = slog.Default()
	}
	e := &Element{BaseElement: core.NewBaseElement(DataKey, nil, logger)}
	keyIndex := map[string]int{}
	readIndex := map[string]int{}
	var properties []core.PropertyMetadata
	for _, m := range models {
		s := &compiledScript{model: m}
		for _, p := range m.properties {
			id := p.key + "|" + string(p.typ)
			index, ok := readIndex[id]
			if !ok {
				key := -1
				if p.elementKey() != DataKey {
					var known bool
					if key, known = keyIndex[p.elementKey()]; !known {
						key = len(e.keys)
						keyIndex[p.elementKey()] = key
						e.keys = append(e.keys, p.elementKey())
					}
				}
				index = len(e.reads)
				readIndex[id] = index
				e.reads = append(e.reads, sourceRead{property: p, key: key})
			}
			s.reads = append(s.reads, index)
		}
		e.scripts = append(e.scripts, s)
		e.metadata = append(e.metadata, m.metadata())
		category := ""
		if m.output.category != nil {
			category = *m.output.category
		}
		properties = append(properties, core.PropertyMetadata{
			Name:        m.output.name,
			Element:     e,
			Category:    category,
			Type:        goType(m.output.valueType),
			Available:   true,
			Description: m.output.description,
		})
		logScript(logger, m)
	}
	e.SetProperties(properties)
	return e, nil
}

// duplicates reports two scripts with the same name, or producing the same
// output property, as there is one script per name and one value per output
// property.
func duplicates(models []*scriptModel) []Fault {
	var faults []Fault
	names := map[string]*scriptModel{}
	outputs := map[string]*scriptModel{}
	for _, m := range models {
		if first, ok := names[strings.ToLower(m.name)]; ok {
			faults = append(faults, Fault{Script: m.name, Source: m.source, Path: "Name", Message: fmt.Sprintf(
				"the script '%s' is added more than once, from %s and from %s", m.name, first.source, m.source)})
		} else {
			names[strings.ToLower(m.name)] = m
		}
		if first, ok := outputs[strings.ToLower(m.output.name)]; ok {
			faults = append(faults, Fault{Script: m.name, Source: m.source, Path: "Output.Name", Message: fmt.Sprintf(
				"the output property '%s' is also produced by the script '%s' from %s",
				m.output.name, first.name, first.source)})
		} else {
			outputs[strings.ToLower(m.output.name)] = m
		}
	}
	return faults
}

// logScript writes one information line per script, the compiled model at
// debug level, and a warning carrying the note of a deprecated script.
func logScript(logger *slog.Logger, m *scriptModel) {
	logger.Info(fmt.Sprintf("Derived property script '%s' version %s, format %d, from %s, produces '%s.%s'",
		m.name, m.version, m.format, m.source, DataKey, m.output.name))
	if logger.Enabled(context.Background(), slog.LevelDebug) {
		logger.Debug(fmt.Sprintf("Derived property script '%s' compiled to:\n%s", m.name, m.canonical()))
	}
	if m.deprecated {
		logger.Warn(fmt.Sprintf("Derived property script '%s' is deprecated. %s", m.name, m.deprecationNote))
	}
}

// BuilderName is the name to register NewFromConfiguration under, so that a
// Pipeline configuration can name the element.
const BuilderName = "DerivedPropertyElement"

// NewFromConfiguration builds an Element from the build parameters of a
// Pipeline configuration. Scripts is a list of built-in script names and
// ScriptFiles a list of file paths where wildcards are allowed, each given as
// a comma separated string. Script text from code is not available from
// configuration. Register it with core.BuilderRegistry under BuilderName.
// See https://github.com/51Degrees/specifications/blob/6570727d2af6967390d3c4fbec77b9cf0540d3f3/pipeline-specification/pipeline-elements/derived-property-element.md#configuration
func NewFromConfiguration(params map[string]any, logger *slog.Logger) (core.FlowElement, error) {
	b := NewBuilder().SetLogger(logger)
	for key, value := range params {
		list, err := stringList(value)
		if err != nil {
			return nil, fmt.Errorf("build parameter '%s' %w", key, err)
		}
		switch strings.ToLower(key) {
		case "scripts":
			for _, name := range list {
				b.AddBuiltInScript(BuiltInScript(name))
			}
		case "scriptfiles":
			for _, path := range list {
				b.AddScriptFile(path)
			}
		default:
			return nil, fmt.Errorf("'%s' is not a build parameter of the Derived Property Element. "+
				"Use Scripts for built-in scripts and ScriptFiles for files", key)
		}
	}
	e, err := b.Build()
	if err != nil {
		return nil, err
	}
	return e, nil
}

// stringList reads a list of strings, given as a comma separated string or as
// a list.
func stringList(value any) ([]string, error) {
	var items []string
	switch v := value.(type) {
	case string:
		items = strings.Split(v, ",")
	case []string:
		items = v
	case []any:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("lists %T where a string is needed", item)
			}
			items = append(items, s)
		}
	default:
		return nil, fmt.Errorf("is %T where a comma separated list of strings is needed", value)
	}
	var out []string
	for _, item := range items {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out, nil
}
