package seeders

import (
	_ "embed"
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/customtool"
	"github.com/Aliizi83/vohu/internal/platform/user"
	"gorm.io/gorm"
)

//go:embed customtool_sources/read_file.go.txt
var readFileSource string

//go:embed customtool_sources/write_file.go.txt
var writeFileSource string

//go:embed customtool_sources/edit_file.go.txt
var editFileSource string

//go:embed customtool_sources/list_directory.go.txt
var listDirectorySource string

//go:embed customtool_sources/search_files.go.txt
var searchFilesSource string

//go:embed customtool_sources/find_files.go.txt
var findFilesSource string

var customTools = []struct {
	name         string
	description  string
	paramsSchema string
	source       string
}{
	{"read_file", "Read a file's contents, with line numbers.",
		`{"type":"object","properties":{"path":{"type":"string"},"offset":{"type":"integer"},"limit":{"type":"integer"}},"required":["path"]}`,
		readFileSource},
	{"write_file", "Create a new file or fully overwrite an existing one.",
		`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`,
		writeFileSource},
	{"edit_file", "Replace one exact block of text in an existing file.",
		`{"type":"object","properties":{"path":{"type":"string"},"old_str":{"type":"string"},"new_str":{"type":"string"}},"required":["path","old_str","new_str"]}`,
		editFileSource},
	{"list_directory", "List files and directories at a path.",
		`{"type":"object","properties":{"path":{"type":"string"},"recursive":{"type":"boolean"},"max_depth":{"type":"integer"}}}`,
		listDirectorySource},
	{"search_files", "Search file contents by regex.",
		`{"type":"object","properties":{"pattern":{"type":"string"},"path":{"type":"string"},"file_glob":{"type":"string"},"case_sensitive":{"type":"boolean"}},"required":["pattern"]}`,
		searchFilesSource},
	{"find_files", "Find files by name/glob pattern.",
		`{"type":"object","properties":{"pattern":{"type":"string"},"path":{"type":"string"}},"required":["pattern"]}`,
		findFilesSource},
}

// seedCustomTools gives each built-in file tool source a customtool.Tool +
// initial ToolVersion row, owned by the admin user. Runs after seedUsers.
// The admin role's wildcard "manage" on resourceType "custom_tool" (see
// seedAdminAccess) already covers these — no separate grant needed here.
func seedCustomTools(database *gorm.DB) error {
	var admin user.User
	if err := database.Where("username = ?", DefaultAdminUsername).First(&admin).Error; err != nil {
		return err
	}

	for _, ct := range customTools {
		var existing customtool.Tool
		err := database.Where("name = ?", ct.name).First(&existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		tool := customtool.Tool{
			Name:            ct.name,
			Description:     ct.description,
			ParamsSchema:    ct.paramsSchema,
			Visibility:      customtool.VisibilityPublic,
			CreatedByUserID: admin.ID,
		}
		if err := database.Create(&tool).Error; err != nil {
			return err
		}

		version := customtool.ToolVersion{
			ToolID:          tool.ID,
			Version:         "1.0.0",
			SourceCode:      ct.source,
			CreatedByUserID: admin.ID,
		}
		if err := database.Create(&version).Error; err != nil {
			return err
		}
	}

	return nil
}
