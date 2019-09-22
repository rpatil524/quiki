package wikifier

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
)

type parser struct {
	pos position

	last   byte // last byte
	this   byte // current byte
	next   byte // next byte
	skip   bool // skip next byte
	escape bool // this byte is escaped

	catch catch // current parser catch
	block block // current parser block

	commentLevel int  // comment depth
	braceLevel   int  // brace escape depth
	braceFirst   bool // true when entering a brace escape

	varName            string
	varNotInterpolated bool
	varNegated         bool
}

type position struct {
	line, column int
}

var variableTokens = map[byte]bool{
	'@': true,
	'%': true,
	':': true,
	';': true,
	'-': true,
}

func newParser() *parser {
	mb := newBlock("main", "", nil, nil, position{})
	return &parser{block: mb, catch: mb}
}

func (p *parser) parseLine(line []byte, page *Page) error {
	for i, b := range line {

		// skip this byte
		if p.skip {
			p.skip = false
			continue
		}

		// update column and bytes
		p.pos.column = i + 1
		p.this = b

		if len(line) > i+1 {
			p.next = line[i+1]
		} else {
			p.next = 0
		}

		// handle this byte and give up if error occurred
		if err := p.parseByte(b, page); err != nil {
			return err
		}
	}

	return nil
}

func (p *parser) parseByte(b byte, page *Page) error {
	log.Printf("parseByte(%s, last: %s, next: %s)", string(b), string(p.last), string(p.next))

	// BRACE ESCAPE
	if p.braceLevel != 0 {
		isFirst := p.braceFirst
		p.braceFirst = false

		if b == '{' && !isFirst {
			// increase brace depth
			p.braceLevel++
		} else if b == '}' {
			// decrease brace depth
			p.braceLevel--

			// if this was the last brace, clear the brace escape catch
			if p.braceLevel == 0 {
				p.catch = p.catch.parentCatch()
			}
		}

		// proceed to the next byte if this was the first or last brace
		if isFirst || p.braceLevel == 0 {
			return p.nextByte(b)
		}

		// otherwise, proceed to the catch
		return p.handleByte(b)
	}

	// COMMENTS

	// entrance
	if b == '/' && p.next == '*' {

		// this is escaped
		if p.escape {
			return p.handleByte(b)
		}

		// next byte
		p.commentLevel++
		log.Println("increased comment level to", p.commentLevel)
		return p.nextByte(b)
	}

	// exit
	if b == '*' && p.next == '/' {

		// we weren't in a comment, so handle normally
		if p.commentLevel == 0 {
			return p.handleByte(b)
		}

		// decrease comment level and skip this and next byte
		p.commentLevel--
		log.Println("decreased comment level to", p.commentLevel)
		p.skip = true
		return p.nextByte(b)
	}

	// we're inside a comment; skip to next byte
	if p.commentLevel != 0 {
		return p.nextByte(b)
	}

	// BLOCKS

	if b == '{' {
		// opens a block

		// this is escaped
		if p.escape {
			return p.handleByte(b)
		}

		var blockClasses []string
		var blockType, blockName string

		// if the next char is @, this is {@some_var}
		if p.next == '@' {
			p.skip = true
			blockType = "variable"
		} else {
			var inBlockName, charsScanned int
			lastContent := p.catch.lastString()
			log.Printf("LAST CONTENT: %v", lastContent)

			// if there is no lastContent, give up because the block has no type
			if len(lastContent) == 0 {
				return errors.New("Block has no type")
			}

			// scan the text backward to find the block type and name
			for i := len(lastContent) - 1; i != -1; i-- {
				lastChar := lastContent[i]
				charsScanned++

				// enter/exit block name
				if lastChar == ']' {
					log.Println("entering block name")
					// entering block name
					inBlockName++

					// we just entered the block name
					if inBlockName == 1 {
						continue
					}
				} else if lastChar == '[' {
					log.Println("exiting block name")

					// exiting block name
					inBlockName--

					// we're still in it
					if inBlockName != 1 {
						continue
					}
				}

				// block type/name
				if inBlockName != 0 {
					// we're currently in the block name
					blockName = string(lastChar) + blockName
				} else if matched, _ := regexp.Match(`[\w\-\$\.]`, []byte{lastChar}); matched {
					// this could be part of the block type
					blockType = string(lastChar) + blockType
					continue
				} else if lastChar == '~' && len(blockType) != 0 {
					// tilde terminates block type
					break
				} else if matched, _ := regexp.Match(`\s`, []byte{lastChar}); matched && len(blockType) == 0 {
					// space between things
					continue
				} else {
					// not sure. give this byte back and bail
					log.Printf("giving up due to: %v", string(lastChar))
					charsScanned--
					break
				}
			}

			// overwrite last content with the title and name stripped out
			log.Printf("Setting last content to: %v", lastContent[:len(lastContent)-charsScanned])
			p.catch.setLastContent(lastContent[:len(lastContent)-charsScanned])

			// if the block contains dots, it has classes
			if split := strings.Split(string(blockType), "."); len(split) > 1 {
				blockType, blockClasses = split[0], split[1:]
			}

			// if there is no type at this point, assume it is a map
			if len(blockType) == 0 {
				blockType = "map"
			}

			// if the block type starts with $, it is a model
			if blockType[0] == '$' {
				blockType = blockType[1:]
				blockName = blockType
				blockType = "model"
			}

			// create the block
			log.Printf("Creating block: %s[%s]{}", blockType, blockName)
			block := newBlock(blockType, blockName, blockClasses, p.block, p.pos)

			// TODO: produce a warning if the block has a name but the type does not support it

			// set the current block
			p.block = block
			p.catch = block

			// if the next char is a brace, this is a brace escaped block
			if p.next == '{' {
				p.braceFirst = true
				p.braceLevel++

				// TODO: set the current catch to the brace escape
				// return if catch fails
			}
		}
		return p.nextByte(b)
	}

	if b == '}' {
		// closes a block

		// this is escaped
		if p.escape {
			return p.handleByte(b)
		}

		// we cannot close the main block
		if p.block.blockType() == "main" {
			return errors.New("Attempted to close main block")
		}

		var addContents []interface{}

		// TODO: if/elsif/else statements, {@vars}
		if false {

		} else {
			// normal block. add the block itself
			addContents = []interface{}{p.block}
		}

		// close the block
		p.block.close(p.pos)

		// clear the catch
		p.block = p.block.parentBlock()
		p.catch = p.catch.parentCatch()
		p.catch.appendContent(addContents, p.pos)

		return p.nextByte(b)
	}

	if b == '\\' {
		// the escape will be handled later
		if p.escape {
			return p.handleByte(b)
		}
		return p.nextByte(b)
	}

	// VARIABLES

	if p.block.blockType() == "main" && variableTokens[b] && p.last != '[' {
		log.Println("variable tok", string(b))

		if p.escape {
			return p.handleByte(b)
		}

		// entering a variable declaration
		if (b == '@' || b == '%') && p.catch == p.block {
			log.Println("variable declaration", string(b))

			// disable interpolation if it's %var
			if b == '%' {
				p.varNotInterpolated = true
			}

			// negate the value if -@var
			pfx := string(b)
			if p.last == '-' {
				pfx = string(p.last) + pfx
				p.varNegated = true
			}

			// catch the var name
			catch := newVariableName(pfx, p.pos)
			catch.parent = p.catch
			p.catch = catch

			return p.nextByte(b)
		}

		// terminate variable name, enter value
		if b == ':' && p.catch.catchType() == catchTypeVariableName {
			// starts a variable value

			// fetch var name, clear the catch
			p.varName = p.catch.lastString()
			p.catch = p.catch.parentCatch()

			// no var name
			if len(p.varName) == 0 {
				return errors.New("Variable has no name")
			}
			log.Printf("VALUE VAR NAME: %v", p.varName)

			// now catch the value
			catch := newVariableValue()
			catch.parent = p.catch
			p.catch = catch

			return p.nextByte(b)
		}

		// terminate a boolean
		if b == ';' && p.catch.catchType() == catchTypeVariableName {

			// fetch var name, clear the catch
			p.varName = p.catch.lastString()
			p.catch = p.catch.parentCatch()

			// no var name
			if len(p.varName) == 0 {
				return errors.New("Variable has no name")
			}
			log.Printf("BOOLEAN VAR NAME: %v", p.varName)

			// TODO: set the value
			// TODO: set as false if varNegated

			p.clearVariableState()
			return p.nextByte(b)
		}

		// terminate a string or block variable value
		if b == ';' && p.catch.catchType() == catchTypeVariableValue {

			// fetch content and clear catch
			values := p.catch.content()
			p.catch = p.catch.parentCatch()

			//     my ($var, $val) =
			//         _get_var_parts(delete @$c{ qw(var_name var_value) });
			//     my ($is_string, $no_intplt, $is_negated) = delete @$c{qw(
			//         var_is_string var_no_interpolate var_is_negated
			//     )};

			//     # more than one content? not allowed in variables
			//     return $c->error("Variable can't contain both text and blocks")
			//         if @$var > 1 || @$val > 1;
			//     $var = shift @$var;
			//     $val = shift @$val;
			if len(values) != 1 {
				return fmt.Errorf("Variable '%s' contains both text and blocks", p.varName)
			}

			// we have to also check this here in case it was something like @;
			if len(p.varName) == 0 {
				return errors.New("Variable has no name")
			}

			switch val := values[0].(type) {
			case string:
				log.Println("Got var str:", val)
				// TODO: Format it

			case block:
				log.Println("Got var block:", val)

			default:
				return fmt.Errorf("Not sure what to do with: %v", val)
			}

			// TODO: set the value
			//     # set the value

			// TODO:
			//     # run ->parse and ->html if necessary
			//     _parse_vars($page, 'parse', $val);
			//     _parse_vars($page, 'html',  $val);

			p.clearVariableState()
			return p.nextByte(b)
		}

		// negates a boolean variable
		if b == '-' && (p.next == '@' || p.next == '%') {
			// do nothing yet; just make sure we don't get to default
			return p.nextByte(b)
		}

		return p.nextByte(b)
	}

	return p.handleByte(b)
}

// (NEXT DEFAULT)
func (p *parser) handleByte(b byte) error {
	log.Println("handleByte", string(b))

	// if we have someplace to append this content, do that
	if p.catch == nil {
		// nothing to catch! I don't think this can ever happen since the main block
		// is the top-level catch and cannot be closed, but it's here just in case
		return errors.New("Nothing to catch byte: " + string(b))
	}

	// at this point, anything that needs escaping should have been handled.
	// so, if this byte is escaped and reached all the way to here, we will
	// pretend it's not escaped by reinjecting a backslash. this allows
	// further parsers to handle escapes (in particular, Formatter.)
	add := string(b)
	if p.escape {
		add = string([]byte{p.last, b})
	}

	// terminate the catch if the catch says to skip this byte
	if p.catch.shouldSkipByte(b) {

		// fetch the stuff caught up to this point
		pc := p.catch.posContent()

		// also, fetch prefixes if there are any
		if pfx := p.catch.positionedPrefixContent(); pfx != nil {
			pc = append(pfx, pc...)
		}

		// revert to the parent catch, and add our stuff to it
		p.catch = p.catch.parentCatch()
		p.catch.pushContents(pc)

	} else if !p.catch.byteOK(b) {
		// ask the catch if this byte is acceptable

		char := string(b)
		if char == "\n" {
			char = "\u2424"
		}
		err := "Invalid byte '" + char + "' in " + p.catch.catchType() + "."
		if str := p.catch.lastString(); str != "" {
			err += "Partial: " + str
		}
		return errors.New(err)
	}

	// append
	p.catch.appendString(add, p.pos)

	return p.nextByte(b)
}

// (NEXT BYTE)
func (p *parser) nextByte(b byte) error {
	log.Println("nextByte", string(b))

	p.last = b

	// if current byte is \, set escape for the next
	if b == '\\' && !p.escape && p.braceLevel == 0 {
		p.escape = true
	} else {
		p.escape = false
	}

	return nil
}

func (p *parser) clearVariableState() {
	p.varName = ""
	p.varNotInterpolated = false
	p.varNegated = false
}