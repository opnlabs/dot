package utils

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

// Compress takes a path to a file or directory and creates a .tar.gzip file
// at the outputPath location
func Compress(path, outputPath string) error {
	tarFile, err := os.Create(outputPath)
	if err != nil {
		log.Fatal(err)
	}
	defer tarFile.Close()

	gzw := gzip.NewWriter(tarFile)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	return filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(path)
		if err := tw.WriteHeader(header); err != nil {
			log.Println(err)
			return err
		}

		if !info.IsDir() {
			data, err := os.Open(path)
			if err != nil {
				log.Println(err)
				return err
			}
			if _, err := io.Copy(tw, data); err != nil {
				log.Println(err, header.Name, header.Size)
				return err
			}
			data.Close()
		}
		return nil
	})
}

// Decompress takes a location to a .tar.gzip file and a base path and
// decompresses the contents wrt the base path
func Decompress(tarPath, baseDir string) error {
	tarFile, err := os.Open(tarPath)
	if err != nil {
		log.Println(err)
		return err
	}
	defer tarFile.Close()

	gzr, err := gzip.NewReader(tarFile)
	if err != nil {
		log.Println(err)
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			log.Println(err)
			return err
		}

		target := filepath.Join(baseDir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, fs.FileMode(header.Mode)); err != nil {
					return err
				}
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, 0755)
			if err != nil {
				log.Println(err)
				return err
			}
			defer f.Close()

			if _, err := io.Copy(f, tr); err != nil {
				log.Println(err)
				return err
			}
		}
	}
}

// TarCopy uses tar archive to copy src to dst to preserve the folder structure
func TarCopy(src, dst, tempDir string) error {
	f, err := os.CreateTemp(tempDir, "tarcopy-*.tar.gzip")
	if err != nil {
		log.Println(err)
		return err
	}
	f.Close()

	if err := Compress(src, f.Name()); err != nil {
		log.Println(err)
		return err
	}

	if err := Decompress(f.Name(), dst); err != nil {
		log.Println(err)
		return err
	}

	return nil
}
