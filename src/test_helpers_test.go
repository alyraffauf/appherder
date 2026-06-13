package main

import "os"

func fileMode(path string) os.FileMode {
	info, err := os.Stat(path)
	if err != nil {
		panic(err)
	}
	return info.Mode() & 0o777
}
