package deploy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/watzon/alyx/internal/schema"
)

// Bundler creates deployment bundles from local project files.
type Bundler struct {
	schemaPath    string
	functionsPath string
}

// NewBundler creates a new bundler.
func NewBundler(schemaPath, functionsPath string) *Bundler {
	return &Bundler{
		schemaPath:    schemaPath,
		functionsPath: functionsPath,
	}
}

// CreateBundle creates a deployment bundle from local files.
func (b *Bundler) CreateBundle() (*Bundle, error) {
	bundle := &Bundle{}

	// Load and hash schema
	if b.schemaPath != "" {
		schemaData, err := os.ReadFile(b.schemaPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("schema file not found: %s", b.schemaPath)
			}
			return nil, fmt.Errorf("reading schema file: %w", err)
		}

		parsedSchema, err := schema.Parse(schemaData)
		if err != nil {
			return nil, fmt.Errorf("parsing schema: %w", err)
		}

		bundle.Schema = parsedSchema
		bundle.SchemaRaw = string(schemaData)
		bundle.SchemaHash = hashBytes(schemaData)
	}

	// Discover and hash functions
	if b.functionsPath != "" {
		functions, err := b.discoverFunctions()
		if err != nil {
			return nil, fmt.Errorf("discovering functions: %w", err)
		}
		bundle.Functions = functions
		bundle.FunctionsHash = b.computeFunctionsHash(functions)
	}

	return bundle, nil
}

// discoverFunctions finds all function files and computes their metadata.
func (b *Bundler) discoverFunctions() ([]*FunctionInfo, error) {
	if _, err := os.Stat(b.functionsPath); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(b.functionsPath)
	if err != nil {
		return nil, fmt.Errorf("reading functions directory: %w", err)
	}

	functions := make([]*FunctionInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Skip hidden files and shared modules
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}

		runtime := detectRuntime(name)
		if runtime == "" {
			continue
		}

		path := filepath.Join(b.functionsPath, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		hash, err := hashFile(path)
		if err != nil {
			continue
		}

		baseName := strings.TrimSuffix(name, filepath.Ext(name))
		functions = append(functions, &FunctionInfo{
			Name:     baseName,
			Runtime:  runtime,
			Hash:     hash,
			Path:     path,
			Size:     info.Size(),
			Modified: info.ModTime().Format("2006-01-02T15:04:05Z"),
		})
	}

	// Sort for consistent ordering
	sort.Slice(functions, func(i, j int) bool {
		return functions[i].Name < functions[j].Name
	})

	return functions, nil
}

// detectRuntime detects the runtime from file extension.
func detectRuntime(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".js", ".mjs", ".cjs":
		return "node"
	case ".py":
		return "python"
	case ".go":
		return "go"
	default:
		return ""
	}
}

// computeFunctionsHash computes a combined hash for all functions.
func (b *Bundler) computeFunctionsHash(functions []*FunctionInfo) string {
	if len(functions) == 0 {
		return ""
	}

	// Create a stable representation for hashing
	parts := make([]string, 0, len(functions))
	for _, f := range functions {
		parts = append(parts, fmt.Sprintf("%s:%s:%s", f.Name, f.Runtime, f.Hash))
	}

	combined := strings.Join(parts, "|")
	return hashString(combined)
}

// ReadFunctionFiles reads all function file contents for transfer.
func (b *Bundler) ReadFunctionFiles(functions []*FunctionInfo) (map[string][]byte, error) {
	files := make(map[string][]byte)

	for _, f := range functions {
		data, err := os.ReadFile(f.Path)
		if err != nil {
			return nil, fmt.Errorf("reading function %s: %w", f.Name, err)
		}
		files[f.Name] = data
	}

	return files, nil
}

// DiffFunctions compares local and remote functions.
func DiffFunctions(local, remote []*FunctionInfo) []*FunctionChange {
	var changes []*FunctionChange

	// Build maps for comparison
	localMap := make(map[string]*FunctionInfo)
	for _, f := range local {
		localMap[f.Name] = f
	}

	remoteMap := make(map[string]*FunctionInfo)
	for _, f := range remote {
		remoteMap[f.Name] = f
	}

	// Find additions and modifications
	for name, localFunc := range localMap {
		if remoteFunc, exists := remoteMap[name]; exists {
			if localFunc.Hash != remoteFunc.Hash {
				changes = append(changes, &FunctionChange{
					Type:    FunctionModify,
					Name:    name,
					Runtime: localFunc.Runtime,
					OldHash: remoteFunc.Hash,
					NewHash: localFunc.Hash,
					Safe:    true,
					Reason:  "Function code changed",
				})
			}
		} else {
			changes = append(changes, &FunctionChange{
				Type:    FunctionAdd,
				Name:    name,
				Runtime: localFunc.Runtime,
				NewHash: localFunc.Hash,
				Safe:    true,
				Reason:  "New function added",
			})
		}
	}

	// Find removals
	for name, remoteFunc := range remoteMap {
		if _, exists := localMap[name]; !exists {
			changes = append(changes, &FunctionChange{
				Type:    FunctionRemove,
				Name:    name,
				Runtime: remoteFunc.Runtime,
				OldHash: remoteFunc.Hash,
				Safe:    false,
				Reason:  "Function removed - may break existing clients",
			})
		}
	}

	// Sort for consistent ordering
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Name < changes[j].Name
	})

	return changes
}

// SerializeFunctions serializes function info to JSON for storage.
func SerializeFunctions(functions []*FunctionInfo) (string, error) {
	if len(functions) == 0 {
		return "[]", nil
	}

	data, err := json.Marshal(functions)
	if err != nil {
		return "", fmt.Errorf("serializing functions: %w", err)
	}
	return string(data), nil
}

// DeserializeFunctions deserializes function info from JSON.
func DeserializeFunctions(data string) ([]*FunctionInfo, error) {
	if data == "" || data == "[]" {
		return nil, nil
	}

	var functions []*FunctionInfo
	if err := json.Unmarshal([]byte(data), &functions); err != nil {
		return nil, fmt.Errorf("deserializing functions: %w", err)
	}
	return functions, nil
}

// hashBytes computes SHA256 hash of bytes.
func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// hashFile computes SHA256 hash of a file.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
