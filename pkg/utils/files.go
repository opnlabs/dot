package utils

import (
	"io/fs"
	"log"
	"path/filepath"
)

func CreateFileMap(path, removePath string) map[string]string {
	fileMap := make(map[string]string)
	filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
		rel, err := filepath.Rel(removePath, path)
		if err != nil {
			log.Println(err)
			return err
		}
		log.Println(rel)
		fileMap[path] = rel
		return nil
	})
	return fileMap
}
