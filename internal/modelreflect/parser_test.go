package modelreflect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseModelsFromDir(t *testing.T) {
	// Create a temporary directory with test models
	tmpDir := t.TempDir()

	modelsDir := filepath.Join(tmpDir, "models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		t.Fatalf("Failed to create models directory: %v", err)
	}

	// Create test model file
	modelFile := filepath.Join(modelsDir, "user.go")
	content := `package models

type User struct {
	ID    uint   ` + "`gorm:\"primaryKey\"`" + `
	Name  string
	Email string ` + "`gorm:\"uniqueIndex\"`" + `
}

//goosegorm:"managed:false"
type Legacy struct {
	ID int
}
`
	if err := os.WriteFile(modelFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write model file: %v", err)
	}

	models, err := ParseModelsFromDir(modelsDir, nil)
	if err != nil {
		t.Fatalf("ParseModelsFromDir failed: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}

	// Check User model
	userModel := findModel(models, "User")
	if userModel == nil {
		t.Fatal("User model not found")
	}
	if !userModel.Managed {
		t.Error("User model should be managed")
	}
	if len(userModel.Fields) != 3 {
		t.Errorf("Expected 3 fields in User, got %d", len(userModel.Fields))
	}

	// Check Legacy model
	legacyModel := findModel(models, "Legacy")
	if legacyModel == nil {
		t.Fatal("Legacy model not found")
	}
	if legacyModel.Managed {
		t.Error("Legacy model should not be managed")
	}
}

func TestParseModelsWithManagedTag(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		t.Fatalf("Failed to create models directory: %v", err)
	}

	modelFile := filepath.Join(modelsDir, "test.go")
	content := `package models

//goosegorm:"managed:true"
type ManagedModel struct {
	ID uint
}

//goosegorm:"managed:false"
type UnmanagedModel struct {
	ID int
}

type DefaultModel struct {
	ID int
}
`
	if err := os.WriteFile(modelFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write model file: %v", err)
	}

	models, err := ParseModelsFromDir(modelsDir, nil)
	if err != nil {
		t.Fatalf("ParseModelsFromDir failed: %v", err)
	}

	managed := findModel(models, "ManagedModel")
	if managed == nil || !managed.Managed {
		t.Error("ManagedModel should be managed")
	}

	unmanaged := findModel(models, "UnmanagedModel")
	if unmanaged == nil || unmanaged.Managed {
		t.Error("UnmanagedModel should not be managed")
	}

	defaultModel := findModel(models, "DefaultModel")
	if defaultModel == nil || !defaultModel.Managed {
		t.Error("DefaultModel should be managed by default")
	}
}

func TestParseModelsWithIgnoreList(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		t.Fatalf("Failed to create models directory: %v", err)
	}

	modelFile := filepath.Join(modelsDir, "test.go")
	content := `package models

type User struct {
	ID uint
}

type Ignored struct {
	ID int
}
`
	if err := os.WriteFile(modelFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write model file: %v", err)
	}

	ignoreList := []string{"Ignored"}
	models, err := ParseModelsFromDir(modelsDir, ignoreList)
	if err != nil {
		t.Fatalf("ParseModelsFromDir failed: %v", err)
	}

	if len(models) != 1 {
		t.Errorf("Expected 1 model (Ignored should be filtered), got %d", len(models))
	}

	if findModel(models, "Ignored") != nil {
		t.Error("Ignored model should not be in results")
	}
}

func TestGetTableName(t *testing.T) {
	model := &ParsedModel{
		Name: "UserProfile",
	}

	tableName := model.GetTableName()
	expected := "user_profile"
	if tableName != expected {
		t.Errorf("Expected table name '%s', got '%s'", expected, tableName)
	}
}

func TestGetTableNameWithCustomTableName(t *testing.T) {
	model := &ParsedModel{
		Name:      "UserProfile",
		TableName: "custom_users",
	}

	tableName := model.GetTableName()
	expected := "custom_users"
	if tableName != expected {
		t.Errorf("Expected table name '%s', got '%s'", expected, tableName)
	}
}

func TestParseTableNameMethod(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		t.Fatalf("Failed to create models directory: %v", err)
	}

	modelFile := filepath.Join(modelsDir, "user.go")
	content := `package models

const AuthUserTableName string = "auth_customuser"

type AuthUser struct {
	ID uint ` + "`gorm:\"primaryKey\"`" + `
}

func (AuthUser) TableName() string {
	return AuthUserTableName
}
`
	if err := os.WriteFile(modelFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write model file: %v", err)
	}

	models, err := ParseModelsFromDir(modelsDir, nil)
	if err != nil {
		t.Fatalf("ParseModelsFromDir failed: %v", err)
	}

	authUser := findModel(models, "AuthUser")
	if authUser == nil {
		t.Fatal("AuthUser model not found")
	}

	tableName := authUser.GetTableName()
	expected := "auth_customuser"
	if tableName != expected {
		t.Errorf("Expected table name '%s', got '%s'", expected, tableName)
	}
}

func TestParseTableNameMethodWithStringLiteral(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		t.Fatalf("Failed to create models directory: %v", err)
	}

	modelFile := filepath.Join(modelsDir, "user.go")
	content := `package models

type User struct {
	ID uint ` + "`gorm:\"primaryKey\"`" + `
}

func (User) TableName() string {
	return "custom_users"
}
`
	if err := os.WriteFile(modelFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write model file: %v", err)
	}

	models, err := ParseModelsFromDir(modelsDir, nil)
	if err != nil {
		t.Fatalf("ParseModelsFromDir failed: %v", err)
	}

	user := findModel(models, "User")
	if user == nil {
		t.Fatal("User model not found")
	}

	tableName := user.GetTableName()
	expected := "custom_users"
	if tableName != expected {
		t.Errorf("Expected table name '%s', got '%s'", expected, tableName)
	}
}

func TestToSnakeCase_IDHandling(t *testing.T) {
	// Test that ID field converts to "id" (lowercase), not "i_d"
	model := &ParsedModel{
		Name: "User",
		Fields: []Field{
			{Name: "ID", Type: "uint"},
			{Name: "UserName", Type: "string"},
		},
	}

	// Check that ID field name converts correctly
	for _, field := range model.Fields {
		colName := toSnakeCase(field.Name)
		if field.Name == "ID" && colName != "id" {
			t.Errorf("Expected 'ID' to convert to 'id', got '%s'", colName)
		}
		if field.Name == "UserName" && colName != "user_name" {
			t.Errorf("Expected 'UserName' to convert to 'user_name', got '%s'", colName)
		}
	}
}

func TestParseIndexesFromGormTag(t *testing.T) {
	tests := []struct {
		name      string
		gormTag   string
		fieldName string
		expected  []IndexInfo
	}{
		{
			name:      "simple index",
			gormTag:   "index:idx_email",
			fieldName: "Email",
			expected: []IndexInfo{
				{Name: "idx_email", Unique: false, Priority: 0},
			},
		},
		{
			name:      "unique index",
			gormTag:   "index:idx_email,unique",
			fieldName: "Email",
			expected: []IndexInfo{
				{Name: "idx_email", Unique: true, Priority: 0},
			},
		},
		{
			name:      "named unique index",
			gormTag:   "uniqueIndex:idx_email_unique",
			fieldName: "Email",
			expected: []IndexInfo{
				{Name: "idx_email_unique", Unique: true, Priority: 0},
			},
		},
		{
			name:      "unnamed unique index",
			gormTag:   "uniqueIndex",
			fieldName: "Email",
			expected: []IndexInfo{
				{Name: "idx_email", Unique: true, Priority: 0},
			},
		},
		{
			name:      "multiple indexes",
			gormTag:   "index:idx_email;index:idx_user_email",
			fieldName: "Email",
			expected: []IndexInfo{
				{Name: "idx_email", Unique: false, Priority: 0},
				{Name: "idx_user_email", Unique: false, Priority: 0},
			},
		},
		{
			name:      "no index",
			gormTag:   "primaryKey",
			fieldName: "ID",
			expected:  []IndexInfo{},
		},
		{
			name:      "empty tag",
			gormTag:   "",
			fieldName: "Name",
			expected:  []IndexInfo{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseIndexesFromGormTag(tt.gormTag, tt.fieldName)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d indexes, got %d", len(tt.expected), len(result))
				return
			}
			for i, idx := range result {
				if idx.Name != tt.expected[i].Name {
					t.Errorf("Index %d: expected name %s, got %s", i, tt.expected[i].Name, idx.Name)
				}
				if idx.Unique != tt.expected[i].Unique {
					t.Errorf("Index %d: expected unique %v, got %v", i, tt.expected[i].Unique, idx.Unique)
				}
			}
		})
	}
}

func TestToSnakeCase_AllUppercase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ID", "id"},                     // All uppercase -> all lowercase
		{"UUID", "uuid"},                 // All uppercase -> all lowercase
		{"API", "api"},                   // All uppercase -> all lowercase
		{"HTTP", "http"},                 // All uppercase -> all lowercase
		{"XML", "xml"},                   // All uppercase -> all lowercase
		{"JSON", "json"},                 // All uppercase -> all lowercase
		{"URL", "url"},                   // All uppercase -> all lowercase
		{"HTML", "html"},                 // All uppercase -> all lowercase
		{"CSS", "css"},                   // All uppercase -> all lowercase
		{"JS", "js"},                     // All uppercase -> all lowercase
		{"UserID", "user_i_d"},           // Mixed case -> snake_case (each uppercase letter gets underscore)
		{"APIKey", "a_p_i_key"},          // Mixed case -> snake_case
		{"HTTPClient", "h_t_t_p_client"}, // Mixed case -> snake_case
		{"UserName", "user_name"},        // Mixed case -> snake_case
		{"id", "id"},                     // Already lowercase
		{"userId", "user_id"},            // camelCase -> snake_case (I is uppercase)
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toSnakeCase(tt.input)
			if result != tt.expected {
				t.Errorf("toSnakeCase(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsAllUppercase(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"ID", true},
		{"UUID", true},
		{"API", true},
		{"HTTP", true},
		{"XML", true},
		{"JSON", true},
		{"URL", true},
		{"UserID", false},
		{"APIKey", false},
		{"id", false},
		{"userId", false},
		{"", false},
		{"ID123", false},  // Contains numbers
		{"ID_Key", false}, // Contains underscore
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isAllUppercase(tt.input)
			if result != tt.expected {
				t.Errorf("isAllUppercase(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestShouldIgnore(t *testing.T) {
	model := &ParsedModel{
		Name: "User",
	}

	ignoreList := []string{"User", "Post"}
	if !model.ShouldIgnore(ignoreList) {
		t.Error("User should be ignored")
	}

	ignoreList = []string{"Post"}
	if model.ShouldIgnore(ignoreList) {
		t.Error("User should not be ignored")
	}
}

func TestParseFields(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		t.Fatalf("Failed to create models directory: %v", err)
	}

	modelFile := filepath.Join(modelsDir, "test.go")
	content := `package models

type User struct {
	ID    uint   ` + "`gorm:\"primaryKey\"`" + `
	Name  string
	Email string ` + "`gorm:\"uniqueIndex\"`" + `
	Age   int    ` + "`goosegorm:\"managed:false\"`" + `
}
`
	if err := os.WriteFile(modelFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write model file: %v", err)
	}

	models, err := ParseModelsFromDir(modelsDir, nil)
	if err != nil {
		t.Fatalf("ParseModelsFromDir failed: %v", err)
	}

	user := findModel(models, "User")
	if user == nil {
		t.Fatal("User model not found")
	}

	// Check field parsing
	if len(user.Fields) != 4 {
		t.Errorf("Expected 4 fields, got %d", len(user.Fields))
	}

	idField := findField(user.Fields, "ID")
	if idField == nil {
		t.Fatal("ID field not found")
	}
	if idField.Type != "uint" {
		t.Errorf("Expected ID type 'uint', got '%s'", idField.Type)
	}
	if !contains(idField.GormTag, "primaryKey") {
		t.Error("ID should have primaryKey tag")
	}
}

func findModel(models []ParsedModel, name string) *ParsedModel {
	for i := range models {
		if models[i].Name == name {
			return &models[i]
		}
	}
	return nil
}

func findField(fields []Field, name string) *Field {
	for i := range fields {
		if fields[i].Name == name {
			return &fields[i]
		}
	}
	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
