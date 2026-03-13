package models_test

import (
	"testing"

	"github.com/yylego/gormcngen"
	"github.com/yylego/kratos-examples/demo2kratos/internal/pkg/models"
	"github.com/yylego/osexistpath/osmustexist"
	"github.com/yylego/runpath/runtestpath"
)

// Auto generate columns with go generate command
// Support execution via: go generate ./...
// Delete this comment block if auto generation is not needed
//
//go:generate go test -v -run TestGenerateColumns
func TestGenerateColumns(t *testing.T) {
	// Retrieve the absolute path of the source file based on current test file location
	absPath := osmustexist.FILE(runtestpath.SrcPath(t))
	t.Log(absPath)

	// Define data objects used in column generation - supports both instance and non-instance types
	objects := []any{
		&models.Article{},
	}

	// Configure generation options with latest best practices
	options := gormcngen.NewOptions().
		WithColumnClassExportable(true). // Generate exportable column class names like ArticleColumns
		WithColumnsMethodRecvName("c").  // Set receiver name for column methods
		WithColumnsCheckFieldType(true)  // Enable field type checking for type safe

	// Create configuration and generate code to target file
	cfg := gormcngen.NewConfigs(objects, options, absPath)
	cfg.Gen() // Generate code to "gormcnm.gen.go" file
}
