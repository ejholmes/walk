package main

type Storage interface {
}

// FileStore implements the Storage interface, backed by a file system.
type FileStorage struct {
}
