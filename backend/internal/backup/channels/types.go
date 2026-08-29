package channels

import "context"

type ID string

const OpenList ID = "openlist"
const S3 ID = "s3"

type File struct {
	Name       string
	Size       int64
	ModifiedAt string
	Directory  bool
}

type UploadProgress struct {
	BytesTransferred int64
	TotalBytes       int64
	BytesPerSecond   float64
}

type UploadProgressFunc func(UploadProgress)

type Client interface {
	ID() ID
	Test(context.Context) error
	List(context.Context) ([]File, error)
	Upload(context.Context, string, string) (File, error)
	UploadMetadata(context.Context, string, string) (File, error)
	Download(context.Context, string, string) error
}

type ProgressClient interface {
	Client
	UploadWithProgress(context.Context, string, string, UploadProgressFunc) (File, error)
	UploadMetadataWithProgress(context.Context, string, string, UploadProgressFunc) (File, error)
}
