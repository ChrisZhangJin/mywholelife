package server

import "regexp"

var idRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func validKey(s string) bool {
	return idRe.MatchString(s)
}
