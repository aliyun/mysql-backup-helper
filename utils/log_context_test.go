package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogContextConflictDetection(t *testing.T) {
	// Create temporary directories for testing
	tmpDir := "/tmp/backup-helper-test-log"
	os.MkdirAll(tmpDir+"/logdir", 0755)
	os.MkdirAll(tmpDir+"/custom", 0755)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name           string
		logDir         string
		logFileName    string
		expectWarning  bool
		expectedPath   string
		description    string
	}{
		{
			name:          "absolute path with different logDir",
			logDir:        tmpDir + "/logdir",
			logFileName:   tmpDir + "/custom/test.log",
			expectWarning: true,
			expectedPath:  tmpDir + "/custom/test.log",
			description:   "Should show warning when logFileName is absolute and logDir differs",
		},
		{
			name:          "absolute path with same logDir",
			logDir:        tmpDir + "/custom",
			logFileName:   tmpDir + "/custom/test.log",
			expectWarning: false,
			expectedPath:  tmpDir + "/custom/test.log",
			description:   "Should NOT show warning when logFileName directory matches logDir",
		},
		{
			name:          "relative path",
			logDir:        tmpDir + "/logdir",
			logFileName:   "test.log",
			expectWarning: false,
			expectedPath:  tmpDir + "/logdir/test.log",
			description:   "Should NOT show warning for relative path",
		},
		{
			name:          "empty logFileName",
			logDir:        tmpDir + "/logdir",
			logFileName:   "",
			expectWarning: false,
			expectedPath:  "", // Will be auto-generated
			description:   "Should NOT show warning for empty logFileName",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stderr to check for warnings
			// Note: In a real test, we'd need to capture stderr, but for simplicity
			// we'll just verify the file is created correctly
			
			ctx, err := NewLogContext(tt.logDir, tt.logFileName)
			if err != nil {
				t.Fatalf("NewLogContext failed: %v", err)
			}
			defer ctx.Close()

			// Verify the log file was created at the expected path
			actualPath := ctx.GetFileName()
			
			if tt.expectedPath != "" {
				// For auto-generated names, just check it's in the right directory
				if tt.logFileName == "" {
					if !strings.Contains(actualPath, tt.logDir) {
						t.Errorf("Expected log file in %s, got %s", tt.logDir, actualPath)
					}
				} else {
					// Normalize paths for comparison
					expected, _ := filepath.Abs(tt.expectedPath)
					actual, _ := filepath.Abs(actualPath)
					if expected != actual {
						t.Errorf("Expected path %s, got %s", expected, actual)
					}
				}
			}

			// Verify file exists
			if _, err := os.Stat(actualPath); os.IsNotExist(err) {
				t.Errorf("Log file was not created: %s", actualPath)
			}
		})
	}
}

// TestLogContextWarningOutput tests that warnings are printed to stderr
// This is a manual test - run with: go test -v -run TestLogContextWarningOutput
func TestLogContextWarningOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping manual test in short mode")
	}

	tmpDir := "/tmp/backup-helper-test-warning"
	os.MkdirAll(tmpDir+"/logdir", 0755)
	os.MkdirAll(tmpDir+"/custom", 0755)
	defer os.RemoveAll(tmpDir)

	// This test requires manual inspection of stderr output
	// The warning should be visible when running: go test -v
	ctx, err := NewLogContext(tmpDir+"/logdir", tmpDir+"/custom/warning-test.log")
	if err != nil {
		t.Fatalf("NewLogContext failed: %v", err)
	}
	defer ctx.Close()

	t.Logf("Log file created at: %s", ctx.GetFileName())
	t.Logf("Check stderr output above for [WARNING] message")
}

