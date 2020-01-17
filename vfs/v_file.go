package vfs

import "io"

type VFile interface {
	io.ReadWriteCloser
}
