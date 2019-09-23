package wikifier

import (
	htmlfmt "html"
	"strconv"
	"strings"
)

var identifiers = make(map[string]int)

type Html string

type element interface {

	// tag
	tag() string
	setTag(tag string)

	// attributes
	hasAttr(name string) bool
	attr(name string) string
	boolAttr(name string) bool
	setAttr(name, value string)
	setBoolAttr(name string, value bool)

	// metadata
	hasMeta(name string) bool
	meta(name string) string

	// adding content
	addText(s string)
	addHtml(h Html)
	addChild(child element)
	createChild(tag, typ string) element

	// classes
	addClass(class string)
	removeClass(class string) bool

	// parent
	parent() element
	setParent(parent element)
	setNeedID(need bool)

	// html generation
	generate() Html
}

type genericElement struct {
	_tag          string                 // html tag
	attrs         map[string]interface{} // html attributes
	style         map[string]string      // inline styles
	metas         map[string]string      // metadata
	id            string                 // unique element identifier
	typ           string                 // primary quiki class
	classes       []string               // quiki user-defined classes
	content       []interface{}          // mixed text and child elements
	parentElement element                // parent element, if any
	cachedHTML    Html                   // cached version
	container     bool                   // true for container elements
	needID        bool                   // true if we should include id
	noTags        bool                   // if true, only generate inner HTML
	noIndent      bool                   // if true, do not indent contents (for <pre>)
	noClose       bool                   // if true, do not close (containers only)
}

func newElement(tag, typ string) element {
	identifiers[typ]++
	return &genericElement{
		_tag:      tag,
		id:        typ + "-" + strconv.Itoa(identifiers[typ]),
		typ:       typ,
		container: tag == "div",
	}
}

// fetch tag
func (el *genericElement) tag() string {
	return el._tag
}

// set the tag
func (el *genericElement) setTag(tag string) {
	el._tag = tag
}

// true when a meta key is present on an element
func (el *genericElement) hasMeta(name string) bool {
	_, exist := el.metas[name]
	return exist
}

// fetch string value for an attribute
func (el *genericElement) meta(name string) string {
	return el.metas[name]
}

// true when an attr is present on an element
func (el *genericElement) hasAttr(name string) bool {
	_, exist := el.attrs[name]
	return exist
}

// fetch string value for an attribute
func (el *genericElement) attr(name string) string {
	attr, exist := el.attrs[name]
	if !exist {
		return ""
	}
	if attrStr, ok := attr.(string); ok {
		return attrStr
	}
	return ""
}

// fetch boolean value for an attribute
func (el *genericElement) boolAttr(name string) bool {
	attr, exist := el.attrs[name]
	if !exist {
		return false
	}
	if attrBool, ok := attr.(bool); ok {
		return attrBool
	}
	return false
}

// set a string attribute
func (el *genericElement) setAttr(name, value string) {
	if value == "" {
		delete(el.attrs, name)
		return
	}
	el.attrs[name] = value
}

// set a boolean attribute
func (el *genericElement) setBoolAttr(name string, value bool) {
	if value == false {
		delete(el.attrs, name)
		return
	}
	el.attrs[name] = true
}

// add a text node
func (el *genericElement) addText(s string) {
	el.content = append(el.content, s)
}

// add inner html
func (el *genericElement) addHtml(h Html) {
	el.content = append(el.content, h)
}

// add child element
func (el *genericElement) addChild(child element) {
	el.content = append(el.content, child)
}

// create a child element and add it
func (el *genericElement) createChild(tag, typ string) element {
	child := newElement(tag, typ)
	el.addChild(child)
	return child
}

// fetch element's parent
func (el *genericElement) parent() element {
	return el.parentElement
}

// set this element's parent (internal only)
func (el *genericElement) setParent(parent element) {
	el.parentElement = parent // recursive!!
}

// set whether to include element's unique ID
func (el *genericElement) setNeedID(need bool) {
	el.needID = need
}

// add a class
func (el *genericElement) addClass(class string) {
	el.classes = append(el.classes, class)
}

// remove a class, returning true if it was present
func (el *genericElement) removeClass(class string) bool {
	for i, v := range el.classes {
		if v == class {
			el.classes = append(el.classes[:i], el.classes[i+1:]...)
			return true
		}
	}
	return false
}

func (el *genericElement) generate() Html {
	generated := ""

	// cached version
	if el.cachedHTML != "" {
		return el.cachedHTML
	}

	// if we haven't yet determined if this is a container,
	// check if it has any child elements
	if !el.container {
		el.container = len(el.content) != 0
	}

	// tags
	if !el.noTags {
		generated = "<" + el._tag

		// classes
		classes := make([]string, len(el.classes)+1)
		classes[0] = "q-" + el.typ
		for i, name := range el.classes {
			classes[i+1] = "qc-" + name
		}

		// inject ID
		if el.needID {
			classes = append([]string{"q-" + el.id}, classes...)
		}
		generated += ` class="` + strings.Join(classes, " ") + `"`

		// styles
		styles := ""
		for key, val := range el.style {
			styles += key + ":" + val + "; "
		}
		if styles != "" {
			generated += ` style="` + styles + `"`
		}

		// other attributes
		for key, val := range el.attrs {
			switch v := val.(type) {
			case string:
				generated += " " + key + `="` + htmlfmt.EscapeString(v) + `"`
			case bool:
				generated += " " + key
			}
		}
	}

	// non-container
	if !el.container {
		generated += " />\n"
		el.cachedHTML = Html(generated)
		return Html(generated)
	}

	// inner content
	generated += ">\n"
	for _, textOrEl := range el.content {
		add := ""
		switch v := textOrEl.(type) {
		case Html:
			add = string(v)
		case string:
			add = htmlfmt.EscapeString(v)
		case *genericElement:
			add = string(v.generate())
		}
		if !el.noIndent {
			add = indent(add)
		}
		generated += add
	}

	// close it off
	if !el.noTags && !el.noClose {
		generated += "</" + el._tag + ">\n"
	}

	el.cachedHTML = Html(generated)
	return Html(generated)
}

func indent(str string) string {
	var res []rune
	bol := true
	for _, c := range str {
		if bol && c != '\n' {
			res = append(res, []rune("    ")...)
		}
		res = append(res, c)
		bol = c == '\n'
	}
	return string(res)
}
