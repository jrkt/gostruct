package gostruct

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"unicode"
)

//exists checks if path or file exists
func exists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}

	return false
}

// uppercaseFirst does exactly what it says it does
func uppercaseFirst(s string) string {
	if len(s) < 2 {
		return strings.ToLower(s)
	}

	bts := []byte(s)

	lc := bytes.ToUpper([]byte{bts[0]})
	rest := bts[1:]

	return string(bytes.Join([][]byte{lc, rest}, nil))
}

// createDirectory creates directory and sets permissions to 0777
func createDirectory(path string) error {
	err := os.Mkdir(path, 0777)
	if err != nil {
		return err
	}

	//give new directory full permissions
	err = os.Chmod(path, 0777)
	if err != nil {
		return err
	}

	return nil
}

// writeFile contents to file and overwrites if specified
func writeFile(path, contents string, overwrite bool) error {
	var err error
	if exists(path) && overwrite {
		err = os.Remove(path)
	}

	file, err := os.Create(path)
	defer file.Close()
	if err != nil {
		return err
	}

	_, err = file.WriteString(contents)
	if err != nil {
		return err
	}

	return nil
}

// runCommand runs cli commands
func runCommand(command string) (string, error) {
	parts := getCmdParts(command)
	cmd := exec.Command(parts[0], parts[1:]...)
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return "", nil
}

// getCmdParts normalizes command into a string array
func getCmdParts(command string) []string {
	lastQuote := rune(0)
	f := func(c rune) bool {
		switch {
		case c == lastQuote:
			lastQuote = rune(0)
			return false
		case lastQuote != rune(0):
			return false
		case unicode.In(c, unicode.Quotation_Mark):
			lastQuote = c
			return false
		default:
			return unicode.IsSpace(c)
		}
	}

	var parts []string
	preParts := strings.FieldsFunc(command, f)
	for i := range preParts {
		part := preParts[i]
		parts = append(parts, strings.Replace(part, "'", "", -1))
	}

	return parts
}

// inArray determines if string is in array
func inArray(char string, strings []string) bool {
	for _, a := range strings {
		if a == char {
			return true
		}
	}
	return false
}
