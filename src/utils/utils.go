package utils

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jmcvetta/randutil"
	"gopkg.in/yaml.v1"
)

func JSONDecode(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}

func JSONEncode(v interface{}) (string, error) {
	r, err := json.Marshal(v)
	return string(r), err
}

func YAMLDecode(data string, v interface{}) error {
	return yaml.Unmarshal([]byte(data), v)
}

func YAMLEncode(v interface{}) (string, error) {
	r, err := yaml.Marshal(v)
	return string(r), err
}

func EnsureDir(path string, owner, group int) error {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return err
	}
	return os.Chown(path, owner, group)
}

func EnsureFileAbsent(path string) error {
	return os.Remove(path)
}

func RandomString(length int) string {
	r, _ := randutil.AlphaString(length)
	return r
}

// 把src copy到dst
// dst, src必须是绝对路径
// dst不能是src的子目录, 也就是dst不能有src的前缀
// 同时设置所有权限
func CopyFiles(dst, src string, uid, gid int) error {
	if _, err := os.Stat(src); err != nil {
		return err
	}
	if !(filepath.IsAbs(dst) && filepath.IsAbs(src)) {
		return errors.New("both dst and src should be absolute path")
	}
	if strings.HasPrefix(dst, src) {
		return errors.New("dst can't be child of src")
	}
	if err := EnsureDir(dst, uid, gid); err != nil {
		return err
	}
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		suffix := strings.Replace(p, src, "", 1)
		newPath := path.Join(dst, suffix)
		if info.IsDir() {
			if e := EnsureDir(newPath, uid, gid); e != nil {
				return e
			}
		} else {
			d, e := os.Create(newPath)
			defer d.Close()
			if e != nil {
				return e
			}

			f, e := os.Open(p)
			defer f.Close()
			if e != nil {
				return e
			}

			io.Copy(d, f)
			if e := os.Chown(newPath, uid, gid); e != nil {
				return e
			}
		}
		return err
	})
}

func Atoi(s string, def int) int {
	if r, err := strconv.Atoi(s); err != nil {
		return def
	} else {
		return r
	}
}
