package main

import "path/filepath"

func pluralize(count int, singular, plural string) string {
	if count > 1 {
		return plural
	}
	return singular
}

func abs(wd, path string) string {
	if !filepath.IsAbs(path) {
		path = filepath.Join(wd, path)
	}
	return filepath.Clean(path)
}
