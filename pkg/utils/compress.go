// Package utils provides some utility functions to compress and decompress tar and tar.gz.
// It also provides a logger that can output in color and implements io.Writer.
package utils

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const MaxFileSizeBytes = 50 * 1024 * 1024 // 50MB

// Compress takes a path to a file or directory and creates a .tar.gzip file at the outputPath location.
func Compress(path, outputPath string) error {
	tarFile, err := os.Create(filepath.Clean(outputPath))
	if err != nil {
		return fmt.Errorf("could not create tar.gzip file %s: %v", outputPath, err)
	}
	defer tarFile.Close()

	gzw := gzip.NewWriter(tarFile)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	return filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return fmt.Errorf("could not create tar.gzip file %s: %v", path, err)
		}
		header.Name = filepath.ToSlash(path)
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("could not create tar.gzip file %s: %v", path, err)
		}

		if !info.IsDir() {
			data, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("could not open file %s: %v", path, err)
			}
			if _, err := io.Copy(tw, data); err != nil {
				return fmt.Errorf("could not copy tar.gzip contents for file %s: %v", path, err)
			}
			if err := data.Close(); err != nil {
				return fmt.Errorf("could not close file %s: %v", data.Name(), err)
			}
		}
		return nil
	})
}

// Decompress takes a location to a .tar.gzip file and a base path and decompresses the contents wrt the base path.
func Decompress(tarPath, baseDir string) error {
	tarFile, err := os.Open(filepath.Clean(tarPath))
	if err != nil {
		return fmt.Errorf("could not open tar.gzip file %s: %v", tarPath, err)
	}
	defer tarFile.Close()

	gzr, err := gzip.NewReader(tarFile)
	if err != nil {
		return fmt.Errorf("could not read tar.gzip file %s: %v", tarPath, err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("could not read tar.gzip header %s: %v", header.Name, err)
		}

		target, err := sanitizeArchivePath(baseDir, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, fs.FileMode(header.Mode)); err != nil {
					return fmt.Errorf("could not create dir %s: %v", target, err)
				}
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, 0644)
			if err != nil {
				return fmt.Errorf("could not open file %s: %v", target, err)
			}
			defer f.Close()

			if _, err := io.CopyN(f, tr, MaxFileSizeBytes); err != nil && err != io.EOF {
				return fmt.Errorf("could not copy tar.gzip contents to file %s: %v", target, err)
			}
		}
	}
}

// CompressTar takes a path to a file or directory and creates a .tar file at the outputPath location.
func CompressTar(path, outputPath string) error {
	tarFile, err := os.Create(filepath.Clean(outputPath))
	if err != nil {
		return fmt.Errorf("could not create tar.gzip file %s: %v", outputPath, err)
	}
	defer tarFile.Close()

	tw := tar.NewWriter(tarFile)
	defer tw.Close()

	return filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return fmt.Errorf("could not create tar.gzip file %s: %v", path, err)
		}
		header.Name = filepath.ToSlash(path)
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("could not create tar.gzip file %s: %v", path, err)
		}

		if !info.IsDir() {
			data, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("could not open file %s: %v", path, err)
			}
			if _, err := io.Copy(tw, data); err != nil {
				return fmt.Errorf("could not copy tar.gzip contents for file %s: %v", path, err)
			}
			if err := data.Close(); err != nil {
				return fmt.Errorf("could not close file %s: %v", data.Name(), err)
			}
		}
		return nil
	})
}

// DecompressTar takes a location to a .tar file and a base path and decompresses the contents wrt the base path.
func DecompressTar(tarPath, baseDir string) error {
	tarFile, err := os.Open(filepath.Clean(tarPath))
	if err != nil {
		return fmt.Errorf("could not open tar file %s: %v", tarPath, err)
	}
	defer tarFile.Close()

	tr := tar.NewReader(tarFile)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("could not read tar header %s: %v", header.Name, err)
		}

		target, err := sanitizeArchivePath(baseDir, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, fs.FileMode(header.Mode)); err != nil {
					return fmt.Errorf("could not create dir %s: %v", target, err)
				}
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, 0644)
			if err != nil {
				return fmt.Errorf("could not open file %s: %v", target, err)
			}
			defer f.Close()

			if _, err := io.CopyN(f, tr, MaxFileSizeBytes); err != nil && err != io.EOF {
				return fmt.Errorf("could not copy tar contents to file %s: %v", target, err)
			}
		}
	}
}

// TarCopy uses tar archive to copy src to dst to preserve the folder structure.
func TarCopy(src, dst, tempDir string) error {
	f, err := os.CreateTemp(tempDir, "tarcopy-*.tar.gzip")
	if err != nil {
		return fmt.Errorf("could not create tar.gzip file in %s: %v", tempDir, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("could not close file %s: %v", f.Name(), err)
	}

	if err := Compress(src, f.Name()); err != nil {
		return fmt.Errorf("could not create %s from src %s: %v", f.Name(), src, err)
	}

	if err := Decompress(f.Name(), dst); err != nil {
		return fmt.Errorf("could not decompress %s to dst %s: %v", f.Name(), dst, err)
	}

	return nil
}

func sanitizeArchivePath(dir, target string) (string, error) {
	full := filepath.Join(dir, target)
	if strings.HasPrefix(full, filepath.Clean(dir)) {
		return full, nil
	}

	return "", fmt.Errorf("illegal path %s", full)
}
