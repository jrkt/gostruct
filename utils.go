package gostruct

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
