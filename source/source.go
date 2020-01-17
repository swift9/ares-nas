package source

import "io"

type Source interface {
	io.ReadCloser
	MD5() (string, error)
}

type UrlSource struct {
}

func (s *UrlSource) Read(bs []byte) (int, error) {

	return 0, nil
}

func (s *UrlSource) Close() error {
	return nil
}

func (s *UrlSource) MD5() error {
	return nil
}

func NewUrlSource() {

}
