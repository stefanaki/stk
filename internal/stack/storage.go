package stack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	stacksDir      = "stacks"
	currentFile    = "current"
	stackExtension = ".yaml"
)

// Storage handles persistence of stacks to disk.
type Storage struct {
	gitDir string
}

// NewStorage creates a new storage instance for the given git directory.
func NewStorage(gitDir string) *Storage {
	return &Storage{gitDir: gitDir}
}

// stacksPath returns the path to the stacks directory.
func (s *Storage) stacksPath() string {
	return filepath.Join(s.gitDir, stacksDir)
}

// stackPath returns the path to a specific stack file.
func (s *Storage) stackPath(name string) string {
	return filepath.Join(s.stacksPath(), name+stackExtension)
}

// currentPath returns the path to the current stack marker file.
func (s *Storage) currentPath() string {
	return filepath.Join(s.stacksPath(), currentFile)
}

// EnsureDir ensures the stacks directory exists.
func (s *Storage) EnsureDir() error {
	return os.MkdirAll(s.stacksPath(), 0755)
}

// Save persists a stack to disk.
func (s *Storage) Save(stack *Stack) error {
	if err := s.EnsureDir(); err != nil {
		return fmt.Errorf("failed to create stacks directory: %w", err)
	}

	data, err := yaml.Marshal(stack)
	if err != nil {
		return fmt.Errorf("failed to marshal stack: %w", err)
	}

	path := s.stackPath(stack.Name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write stack file: %w", err)
	}

	return nil
}

// Load reads a stack from disk.
func (s *Storage) Load(name string) (*Stack, error) {
	path := s.stackPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("stack %q not found", name)
		}
		return nil, fmt.Errorf("failed to read stack file: %w", err)
	}

	var stack Stack
	if err := yaml.Unmarshal(data, &stack); err != nil {
		return nil, fmt.Errorf("failed to parse stack file: %w", err)
	}

	return &stack, nil
}

// Delete removes a stack from disk.
func (s *Storage) Delete(name string) error {
	path := s.stackPath(name)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("stack %q not found", name)
		}
		return fmt.Errorf("failed to delete stack file: %w", err)
	}

	// If this was the current stack, clear the current marker
	current, _ := s.GetCurrent()
	if current == name {
		_ = os.Remove(s.currentPath())
	}

	return nil
}

// Exists checks if a stack exists.
func (s *Storage) Exists(name string) bool {
	_, err := os.Stat(s.stackPath(name))
	return err == nil
}

// List returns all stack names.
func (s *Storage) List() ([]string, error) {
	entries, err := os.ReadDir(s.stacksPath())
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read stacks directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, stackExtension) {
			names = append(names, strings.TrimSuffix(name, stackExtension))
		}
	}

	return names, nil
}

// SetCurrent marks a stack as the current active stack.
func (s *Storage) SetCurrent(name string) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}

	if name != "" && !s.Exists(name) {
		return fmt.Errorf("stack %q not found", name)
	}

	path := s.currentPath()
	if name == "" {
		return os.Remove(path)
	}

	return os.WriteFile(path, []byte(name), 0644)
}

// GetCurrent returns the name of the current active stack.
func (s *Storage) GetCurrent() (string, error) {
	data, err := os.ReadFile(s.currentPath())
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read current stack: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}

// LoadCurrent loads the current active stack.
func (s *Storage) LoadCurrent() (*Stack, error) {
	name, err := s.GetCurrent()
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("no active stack; run 'stk init <name>' or 'stk switch <name>'")
	}
	return s.Load(name)
}

// Rename renames a stack.
func (s *Storage) Rename(oldName, newName string) error {
	if !s.Exists(oldName) {
		return fmt.Errorf("stack %q not found", oldName)
	}
	if s.Exists(newName) {
		return fmt.Errorf("stack %q already exists", newName)
	}

	stack, err := s.Load(oldName)
	if err != nil {
		return err
	}

	stack.Name = newName
	if err := s.Save(stack); err != nil {
		return err
	}

	if err := os.Remove(s.stackPath(oldName)); err != nil {
		return fmt.Errorf("failed to remove old stack file: %w", err)
	}

	// Update current if needed
	current, _ := s.GetCurrent()
	if current == oldName {
		return s.SetCurrent(newName)
	}

	return nil
}
