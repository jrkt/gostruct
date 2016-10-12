package gostruct

import (
	"os"
	"strings"
	"bytes"
	"log"
	"os/exec"
	"unicode"
)

func exists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	} else {
		return false
	}
}

func uppercaseFirst(s string) string {

	if len(s) < 2 {
		return strings.ToLower(s)
	}

	bts := []byte(s)

	lc := bytes.ToUpper([]byte{bts[0]})
	rest := bts[1:]

	return string(bytes.Join([][]byte{lc, rest}, nil))
}

func check(e error) {
	if e != nil {
		log.Fatalln(e.Error())
	}
}

func writeFile(path string, contents string, overwrite bool) error {
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

/*
 Run commands as if from the command line
 */
func runCommand(command string, showOutput bool, returnOutput bool) (string, error) {
	if showOutput {
		log.Println("Running command: " + command)
	}

	parts := getCmdParts(command)
	if returnOutput {
		data, err := exec.Command(parts[0], parts[1:]...).Output()
		if err != nil {
			return "", err
		}
		return string(data), nil
	} else {
		cmd := exec.Command(parts[0], parts[1:]...)
		if showOutput {
			cmd.Stderr = os.Stderr
			cmd.Stdout = os.Stdout
		}
		err := cmd.Run()
		if err != nil {
			return "", err
		}
	}

	return "", nil
}

/*
 Normalize command into a string array
 */
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

func inArray(char string, strings []string) bool {
	for _, a := range strings {
		if a == char {
			return true
		}
	}
	return false
}