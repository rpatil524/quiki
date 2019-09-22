package wikifier

import (
	"errors"
	"strings"
)

type attributedObject interface {
	MainBlock() block

	Get(key string) (interface{}, error)
	GetStr(key string) (string, error)
	GetObj(key string) (attributedObject, error)

	Set(key string, value interface{}) (interface{}, error)
	SetStr(key, value string) error
	SetObj(key string, value attributedObject) error

	// internal use
	setOwn(key string, value interface{})
	getOwn(key string) interface{}
}

type variableScope struct {
	vars map[string]interface{}
}

func newVariableScope() *variableScope {
	return &variableScope{make(map[string]interface{})}
}

func (scope *variableScope) MainBlock() block {
	return nil
}

func (scope *variableScope) Set(key string, value interface{}) (interface{}, error) {
	var where attributedObject = scope
	// my @parts   = split /\./, $var;
	// my $setting = pop @parts;

	parts := strings.Split(key, ".")
	setting, parts := parts[len(parts)-1], parts[:len(parts)-1]

	// while (length($var = shift @parts)) {
	for _, name := range parts {

		//     my ($new_where, $err) = _get_attr($where, $var);
		//     return (undef, $err) if $err
		newWhere, err := where.GetObj(name)
		if err != nil {
			return "", err
		}

		// this location doesn't exist; make a new map
		if newWhere == nil {
			newWhere = NewMap(where.MainBlock())
			// TODO: maybe somehow include some positioning info here?
		}

		where = newWhere
	}

	where.setOwn(setting, value)
	return value, nil
}

func (scope *variableScope) SetStr(key, value string) error {
	scope.vars[key] = value
	return nil
}

func (scope *variableScope) SetObj(key string, value attributedObject) error {
	scope.vars[key] = value
	return nil
}

// fetch a variable regardless of type
// only fails if attempting to fetch attributes on non attributed value
// does not fail due to the absence of a value
func (scope *variableScope) Get(key string) (interface{}, error) {
	var where attributedObject = scope

	parts := strings.Split(key, ".")
	setting, parts := parts[len(parts)-1], parts[:len(parts)-1]

	for _, name := range parts {
		newWhere, err := where.GetObj(name)
		if err != nil {
			return nil, err
		}
		where = newWhere
	}

	return where.getOwn(setting), nil
}

// fetch the string value of a variable
// fails only if a non-string value is present
func (scope *variableScope) GetStr(key string) (string, error) {
	val, err := scope.Get(key)
	if err != nil {
		return "", err
	}

	// there is nothing here
	if val == nil {
		return "", nil
	}

	// something is here, so it best be a string
	if str, ok := val.(string); ok {
		return str, nil
	}

	// not what we asked for
	return "", errors.New("not a string")
}

// fetch the object value of a variable
// fails only if a non-object value is present
func (scope *variableScope) GetObj(key string) (attributedObject, error) {
	obj, err := scope.Get(key)
	if err != nil {
		return nil, err
	}

	// there is nothing here
	if obj == nil {
		return nil, nil
	}

	// something is here, so it best be an attributedObject
	if aObj, ok := obj.(attributedObject); ok {
		return aObj, nil
	}

	// not what we asked for
	return nil, errors.New("not an object")
}

// INTERNAL

func (scope *variableScope) setOwn(key string, value interface{}) {
	scope.vars[key] = value
}

func (scope *variableScope) getOwn(key string) interface{} {
	if val, exist := scope.vars[key]; exist {
		return val
	}
	return nil
}
