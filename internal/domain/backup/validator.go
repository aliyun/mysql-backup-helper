package backup

import (
	"bufio"
	"os"

	"github.com/fatih/color"
	"github.com/gioco-play/easy-i18n/i18n"
)

// FileInfo backup file information
type FileInfo struct {
	IsValid      bool   // whether it's a valid xtrabackup xbstream file
	ErrorMessage string // error message
}

// ValidateFile validates if the backup file is a valid xtrabackup xbstream file
func ValidateFile(filePath string) (*FileInfo, error) {
	info := &FileInfo{
		IsValid: false,
	}

	// check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		info.ErrorMessage = i18n.Sprintf("File does not exist: %s", filePath)
		return info, nil
	}

	// open file
	file, err := os.Open(filePath)
	if err != nil {
		info.ErrorMessage = i18n.Sprintf("Cannot open file: %v", err)
		return info, nil
	}
	defer file.Close()

	// read file header to detect xbstream format
	reader := bufio.NewReader(file)
	info.IsValid = validateXbstreamFormat(reader)

	if !info.IsValid {
		info.ErrorMessage = i18n.Sprintf("Invalid xbstream backup file format")
	}

	return info, nil
}

// validateXbstreamFormat validates xbstream format
func validateXbstreamFormat(reader *bufio.Reader) bool {
	// read xbstream header (8-byte magic number)
	header := make([]byte, 8)
	n, err := reader.Read(header)
	if err != nil || n != 8 {
		return false
	}

	// check xbstream magic number: XBSTCK01
	expectedMagic := []byte{'X', 'B', 'S', 'T', 'C', 'K', '0', '1'}
	for i, b := range expectedMagic {
		if header[i] != b {
			return false
		}
	}

	return true
}

// PrintValidation prints backup file validation results
func PrintValidation(filePath string, info *FileInfo) {
	i18n.Printf("[backup-helper] Validating backup file: %s\n", filePath)

	if info.IsValid {
		i18n.Printf(color.GreenString("[✓] Valid xbstream backup file detected\n"))
	} else {
		i18n.Printf(color.RedString("[✗] Invalid backup file\n"))
		if info.ErrorMessage != "" {
			i18n.Printf(color.RedString("[✗] Error: %s\n"), info.ErrorMessage)
		}
	}
}

// ValidateStdin validates backup file from stdin
// Note: for stdin, we skip validation to avoid data loss
func ValidateStdin() (*FileInfo, error) {
	info := &FileInfo{
		IsValid: true, // assume stdin data is valid
	}

	// for stdin, we skip validation to avoid consuming data
	// user should ensure the stdin data is valid xbstream format
	return info, nil
}

// PrintStdinValidation prints backup file validation results from stdin
func PrintStdinValidation(info *FileInfo) {
	i18n.Printf("[backup-helper] Validating backup data from stdin...\n")

	if info.IsValid {
		i18n.Printf(color.YellowString("[ℹ] Skipping validation for stdin to avoid data consumption\n"))
		i18n.Printf(color.YellowString("[ℹ] Please ensure the stdin data is valid xbstream format\n"))
	} else {
		i18n.Printf(color.RedString("[✗] Invalid backup data\n"))
		if info.ErrorMessage != "" {
			i18n.Printf(color.RedString("[✗] Error: %s\n"), info.ErrorMessage)
		}
	}
}
