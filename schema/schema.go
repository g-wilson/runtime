package jsonschema

import (
	"fmt"
	"io/fs"
	"io/ioutil"

	"github.com/xeipuuv/gojsonschema"
)

// Load attempts to read a json schema file
func Load(loader fs.FS, filepath string) (gojsonschema.JSONLoader, error) {
	file, err := loader.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("cannot open schema file at %s: %w", filepath, err)
	}

	b, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("cannot read schema file at %s: %w", filepath, err)
	}

	return gojsonschema.NewBytesLoader(b), nil
}

// MustLoad attempts to read a json schema file but panics if an error occurs
func MustLoad(loader fs.FS, filepath string) gojsonschema.JSONLoader {
	file, err := loader.Open(filepath)
	if err != nil {
		panic(fmt.Errorf("cannot open schema file at %s: %w", filepath, err))
	}

	b, err := ioutil.ReadAll(file)
	if err != nil {
		panic(fmt.Errorf("cannot read schema file at %s: %w", filepath, err))
	}

	return gojsonschema.NewBytesLoader(b)
}
