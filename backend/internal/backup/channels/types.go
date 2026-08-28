package channels

import "context"

type ID string

const OpenList ID = "openlist"

type File struct {
	Name       string
	Size       int64
	ModifiedAt string
	Directory  bool
}

type Client interface {
	ID() ID
	Test(context.Context) error
	List(context.Context) ([]File, error)
	Upload(context.Context, string, string) (File, error)
	UploadMetadata(context.Context, string, string) (File, error)
	Download(context.Context, string, string) error
}
