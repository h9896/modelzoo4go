package modelzoo

import (
	"crypto/rand"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/samuel/go-zookeeper/zk"
)

func (z Zkinfo) createProtectedSequential(path string, data []byte, acl []zk.ACL) (string, error) {
	if err := validatePath(path, true); err != nil {
		return "", err
	}
	c := z.conn
	var guid [16]byte
	_, err := io.ReadFull(rand.Reader, guid[:16])
	if err != nil {
		return "", err
	}
	guidStr := fmt.Sprintf("%x", guid)

	parts := strings.Split(path, "/")
	//parts[len(parts)-1] = fmt.Sprintf("%s%s-%s", protectedPrefix, guidStr, parts[len(parts)-1])
	rootPath := strings.Join(parts[:len(parts)-1], "/")
	protectedPath := strings.Join(parts, "/")

	var newPath string
	for i := 0; i < 3; i++ {
		newPath, err = c.Create(protectedPath, data, zk.FlagEphemeral|zk.FlagSequence, acl)
		switch err {
		case zk.ErrSessionExpired:
			// No need to search for the node since it can't exist. Just try again.
		case zk.ErrConnectionClosed:
			children, _, err := c.Children(rootPath)
			if err != nil {
				return "", err
			}
			for _, p := range children {
				parts := strings.Split(p, "/")
				if pth := parts[len(parts)-1]; strings.HasPrefix(pth, "_c_") {
					if g := pth[len("_c_") : len("_c_")+32]; g == guidStr {
						return rootPath + "/" + p, nil
					}
				}
			}
		case nil:
			return newPath, nil
		default:
			return "", err
		}
	}
	return "", err
}

func validatePath(path string, isSequential bool) error {
	if path == "" {
		return zk.ErrInvalidPath
	}

	if path[0] != '/' {
		return zk.ErrInvalidPath
	}

	n := len(path)
	if n == 1 {
		// path is just the root
		return nil
	}

	if !isSequential && path[n-1] == '/' {
		return zk.ErrInvalidPath
	}

	// Start at rune 1 since we already know that the first character is
	// a '/'.
	for i, w := 1, 0; i < n; i += w {
		r, width := utf8.DecodeRuneInString(path[i:])
		switch {
		case r == '\u0000':
			return zk.ErrInvalidPath
		case r == '/':
			last, _ := utf8.DecodeLastRuneInString(path[:i])
			if last == '/' {
				return zk.ErrInvalidPath
			}
		case r == '.':
			last, lastWidth := utf8.DecodeLastRuneInString(path[:i])

			// Check for double dot
			if last == '.' {
				last, _ = utf8.DecodeLastRuneInString(path[:i-lastWidth])
			}

			if last == '/' {
				if i+1 == n {
					return zk.ErrInvalidPath
				}

				next, _ := utf8.DecodeRuneInString(path[i+w:])
				if next == '/' {
					return zk.ErrInvalidPath
				}
			}
		case r >= '\u0000' && r <= '\u001f',
			r >= '\u007f' && r <= '\u009f',
			r >= '\uf000' && r <= '\uf8ff',
			r >= '\ufff0' && r < '\uffff':
			return zk.ErrInvalidPath
		}
		w = width
	}
	return nil
}
